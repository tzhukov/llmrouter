package router

import (
	"testing"

	"github.com/user/llmrouter/pkg/config"
)

func TestRouterRegistry(t *testing.T) {
	rr := NewRegistry()

	cfg := &config.Config{
		Agents: map[string]config.AgentConfig{
			"default": {
				Providers: []config.ProviderConfig{
					{Name: "p1", Type: "mock"},
				},
				Routing: config.RoutingConfig{Strategy: "round-robin"},
			},
			"agent-1": {
				Providers: []config.ProviderConfig{
					{Name: "p2", Type: "mock"},
				},
				Routing: config.RoutingConfig{Strategy: "latency"},
			},
		},
	}

	rr.UpdateConfig(cfg)

	// Test default agent
	rDefault := rr.GetRouter("")
	if rDefault == nil {
		t.Fatal("expected default router, got nil")
	}
	if rDefault.strategy != "round-robin" {
		t.Errorf("expected strategy round-robin, got %s", rDefault.strategy)
	}

	// Test specific agent
	r1 := rr.GetRouter("agent-1")
	if r1 == nil {
		t.Fatal("expected agent-1 router, got nil")
	}
	if r1.strategy != "latency" {
		t.Errorf("expected strategy latency, got %s", r1.strategy)
	}

	// Test fallback
	rUnknown := rr.GetRouter("unknown")
	if rUnknown != rDefault {
		t.Errorf("expected unknown to fallback to default, got %v", rUnknown)
	}
}
