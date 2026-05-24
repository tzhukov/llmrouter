package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/user/llmrouter/pkg/config"
	"github.com/user/llmrouter/pkg/router"
)

func TestWatchRemoteConfig(t *testing.T) {
	registry := router.NewRegistry()
	s := &Server{
		Registry: registry,
	}

	agents := map[string]config.AgentConfig{
		"remote-agent": {
			Routing: config.RoutingConfig{Strategy: "latency"},
			Providers: []config.ProviderConfig{
				{Name: "p1", Type: "mock"},
			},
		},
	}
	data, _ := json.Marshal(agents)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.WatchRemoteConfig(ctx, server.URL)

	// Wait for the registry to be updated
	assert.Eventually(t, func() bool {
		r := registry.GetRouter("remote-agent")
		return r != nil && r.GetStrategy() == "latency"
	}, 2*time.Second, 100*time.Millisecond)
}
