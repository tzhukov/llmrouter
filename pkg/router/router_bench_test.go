package router

import (
	"context"
	"testing"

	"github.com/user/llmrouter/pkg/api"
	"github.com/user/llmrouter/pkg/provider/mock"
)

func BenchmarkRouter_ChatCompletion_RoundRobin(b *testing.B) {
	p1 := mock.NewProvider("p1", 0, nil)
	p2 := mock.NewProvider("p2", 0, nil)

	providers := []*ProviderWithMetadata{
		{Provider: p1, Models: []string{"gpt-4"}},
		{Provider: p2, Models: []string{"gpt-4"}},
	}

	r := NewRouter(providers, "round-robin", false, 0)
	ctx := context.Background()
	req := &api.ChatCompletionRequest{Model: "gpt-4"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := r.ChatCompletion(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRouterRegistry_GetRouter(b *testing.B) {
	reg := NewRegistry()
	// No need to populate for a simple Get check since it handles missing keys

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reg.GetRouter("unknown")
	}
}
