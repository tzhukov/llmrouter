package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRemoteProvider_Watch(t *testing.T) {
	agents := map[string]AgentConfig{
		"test-agent": {
			Routing: RoutingConfig{Strategy: "latency"},
		},
	}
	data, _ := json.Marshal(agents)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	}))
	defer server.Close()

	provider := NewRemoteProvider(server.URL)
	updateCh := make(chan map[string]AgentConfig, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go provider.Watch(ctx, updateCh)

	select {
	case received := <-updateCh:
		assert.Equal(t, 1, len(received))
		assert.Equal(t, "latency", received["test-agent"].Routing.Strategy)
	case <-ctx.Done():
		t.Fatal("timed out waiting for update")
	}
}
