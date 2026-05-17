package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/user/llmrouter/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	agentGVR = schema.GroupVersionResource{
		Group:    "llmrouter.io",
		Version:  "v1",
		Resource: "agents",
	}
)

type ConfigServer struct {
	currentConfig []byte
	mu            sync.RWMutex
	clients       map[chan []byte]struct{}
	clientsMu     sync.Mutex
}

func NewConfigServer() *ConfigServer {
	return &ConfigServer{
		clients: make(map[chan []byte]struct{}),
	}
}

func (s *ConfigServer) updateConfig(agents map[string]config.AgentConfig) {
	data, err := json.Marshal(agents)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal agents")
		return
	}

	s.mu.Lock()
	s.currentConfig = data
	s.mu.Unlock()

	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- data:
		default:
			log.Warn().Msg("client buffer full, skipping update")
		}
	}
}

func (s *ConfigServer) handleSync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 1)
	s.clientsMu.Lock()
	s.clients[ch] = struct{}{}
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, ch)
		s.clientsMu.Unlock()
		close(ch)
	}()

	// Send current state first
	s.mu.RLock()
	initialData := s.currentConfig
	s.mu.RUnlock()
	if initialData != nil {
		fmt.Fprintf(w, "data: %s\n\n", initialData)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	for {
		select {
		case data := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(os.Stdout)

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Warn().Msg("not in cluster, attempting to use local kubeconfig...")
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = fmt.Sprintf("%s/.kube/config", os.Getenv("HOME"))
		}
		k8sConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to build local kubernetes config")
		}
	}

	dynClient, err := dynamic.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create dynamic client")
	}

	server := NewConfigServer()

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, time.Minute, metav1.NamespaceAll, nil)
	informer := factory.ForResource(agentGVR).Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			server.reconcile(factory)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			server.reconcile(factory)
		},
		DeleteFunc: func(obj interface{}) {
			server.reconcile(factory)
		},
	})

	stopCh := make(chan struct{})
	defer close(stopCh)
	factory.Start(stopCh)

	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		log.Fatal().Msg("failed to sync cache")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sync", server.handleSync)

	httpServer := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}

	go func() {
		log.Info().Msg("starting config server on :8081")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server failed")
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info().Msg("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
}

func (s *ConfigServer) reconcile(factory dynamicinformer.DynamicSharedInformerFactory) {
	objs, err := factory.ForResource(agentGVR).Lister().List(labels.Everything())
	if err != nil {
		log.Error().Err(err).Msg("failed to list agents")
		return
	}

	agents := make(map[string]config.AgentConfig)
	for _, obj := range objs {
		u := obj.(*unstructured.Unstructured)
		name := u.GetName()

		spec, ok := u.Object["spec"].(map[string]interface{})
		if !ok {
			log.Warn().Str("agent", name).Msg("spec is not a map, skipping")
			continue
		}

		var agent config.AgentConfig
		if r, ok := spec["routing"].(map[string]interface{}); ok {
			agent.Routing.Strategy, _ = r["strategy"].(string)
			if f, ok := r["failover"].(bool); ok {
				agent.Routing.Failover = f
			}
			if rt, ok := r["retries"].(int64); ok {
				agent.Routing.Retries = int(rt)
			}
		}

		if ps, ok := spec["providers"].([]interface{}); ok {
			for _, p := range ps {
				pm, ok := p.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := pm["name"].(string)
				pType, _ := pm["type"].(string)
				if name == "" || pType == "" {
					continue
				}
				pc := config.ProviderConfig{
					Name: name,
					Type: pType,
				}
				if bu, ok := pm["base_url"].(string); ok {
					pc.BaseURL = bu
				}
				if ak, ok := pm["api_key"].(string); ok {
					pc.APIKey = ak
				}
				agent.Providers = append(agent.Providers, pc)
			}
		}
		agents[name] = agent
	}

	s.updateConfig(agents)
}
