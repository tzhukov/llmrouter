package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"github.com/user/llmrouter/pkg/api"
	"github.com/user/llmrouter/pkg/config"
	"github.com/user/llmrouter/pkg/observability"
	"github.com/user/llmrouter/pkg/router"
)

type Server struct {
	Router   *chi.Mux
	Registry *router.RouterRegistry
}

func NewServer() *Server {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(observability.LoggingMiddleware)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	s := &Server{
		Router:   r,
		Registry: router.NewRouterRegistry(),
	}

	s.routes()

	return s
}

func (s *Server) routes() {
	s.Router.Get("/health", s.handleHealth)
	s.Router.Handle("/metrics", promhttp.Handler())
	s.Router.Post("/v1/chat/completions", s.handleChatCompletion)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	var req api.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("failed to decode request")
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	engine := s.Registry.GetRouter(req.AgentID)
	if engine == nil {
		log.Error().Str("agent_id", req.AgentID).Msg("no router found for agent")
		http.Error(w, "no router available for this agent", http.StatusServiceUnavailable)
		return
	}

	if req.Stream {
		s.handleChatCompletionStream(w, r, &req, engine)
		return
	}

	resp, err := engine.ChatCompletion(r.Context(), &req)
	if err != nil {
		log.Error().Err(err).Msg("routing failed")
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleChatCompletionStream(w http.ResponseWriter, r *http.Request, req *api.ChatCompletionRequest, engine *router.Router) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, canFlush := w.(http.Flusher)
	flush := func() {
		if canFlush {
			flusher.Flush()
		}
	}

	respCh, errCh := engine.StreamChatCompletion(r.Context(), req)

	// Use nil-ing pattern to avoid infinite loops on closed channels.
	activeCh := respCh
	activeErrCh := errCh
	for activeCh != nil || activeErrCh != nil {
		select {
		case chunk, ok := <-activeCh:
			if !ok {
				activeCh = nil
				continue
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flush()
		case err, ok := <-activeErrCh:
			if !ok {
				activeErrCh = nil
				continue
			}
			if err != nil {
				log.Error().Err(err).Msg("streaming failed")
				fmt.Fprintf(w, "data: {\"error\":\"%s\"}\n\n", err.Error())
				flush()
				return
			}
		case <-r.Context().Done():
			return
		}
	}

	// All chunks delivered — send OpenAI-compatible stream terminator.
	fmt.Fprint(w, "data: [DONE]\n\n")
	flush()
}

func (s *Server) WatchRemoteConfig(ctx context.Context, url string) {
	provider := config.NewRemoteProvider(url)
	updateCh := make(chan map[string]config.AgentConfig)

	go provider.Watch(ctx, updateCh)

	go func() {
		for {
			select {
			case agents := <-updateCh:
				s.Registry.UpdateAgents(agents)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *Server) Start(addr string) error {
	log.Info().Str("addr", addr).Msg("starting server")
	return http.ListenAndServe(addr, s.Router)
}
