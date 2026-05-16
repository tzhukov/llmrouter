package router_test

import (
	"context"
	"testing"
	"time"

	"github.com/user/llmrouter/pkg/api"
	"github.com/user/llmrouter/pkg/provider/mock"
	"github.com/user/llmrouter/pkg/router"
)

func TestRouter_Strategies(t *testing.T) {
	p1 := &router.ProviderWithMetadata{
		Provider:    mock.NewMockProvider("cheap-but-slow", 50*time.Millisecond, nil),
		PromptPrice: 0.01,
	}
	p2 := &router.ProviderWithMetadata{
		Provider:    mock.NewMockProvider("expensive-but-fast", 10*time.Millisecond, nil),
		PromptPrice: 0.10,
	}

	providers := []*router.ProviderWithMetadata{p1, p2}
	r := router.NewRouter(providers, "round-robin", false, 0)

	ctx := context.Background()
	req := &api.ChatCompletionRequest{Model: "test"}

	t.Run("Cost-based routing", func(t *testing.T) {
		r.SetStrategy("cost")
		resp, err := r.ChatCompletion(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		// Should pick cheap-but-slow (p1)
		expected := "mock-cheap-but-slow"
		if resp.ID[:len(expected)] != expected {
			t.Errorf("expected %s, got %s", expected, resp.ID)
		}
	})

	t.Run("Latency-based routing", func(t *testing.T) {
		r.SetStrategy("latency")
		
		// Warm up: mock-1 is slow, mock-2 is fast
		r.ChatCompletion(ctx, req) // Uses RR initially if no stats? No, RR logic is different.
		// Actually, let's manually update latency to simulate stats
		p1.UpdateLatency(0.050)
		p2.UpdateLatency(0.010)

		resp, err := r.ChatCompletion(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		// Should pick expensive-but-fast (p2)
		expected := "mock-expensive-but-fast"
		if resp.ID[:len(expected)] != expected {
			t.Errorf("expected %s, got %s", expected, resp.ID)
		}
	})
}
