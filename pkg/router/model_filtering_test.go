package router

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/user/llmrouter/pkg/api"
	"github.com/user/llmrouter/pkg/provider/mock"
)

func TestRouter_ModelAwareFiltering(t *testing.T) {
	p1 := &ProviderWithMetadata{
		Provider: mock.NewMockProvider("p1", 0, nil),
		Models:   []string{"gpt-4", "gpt-3.5"},
	}
	p2 := &ProviderWithMetadata{
		Provider: mock.NewMockProvider("p2", 0, nil),
		Models:   []string{"claude-3", "gpt-4"},
	}
	p3 := &ProviderWithMetadata{
		Provider: mock.NewMockProvider("p3", 0, nil),
		Models:   []string{}, // supports all
	}

	r := NewRouter([]*ProviderWithMetadata{p1, p2, p3}, "round-robin", true, 3)
	ctx := context.Background()

	t.Run("Filter by specific model (gpt-3.5)", func(t *testing.T) {
		req := &api.ChatCompletionRequest{Model: "gpt-3.5"}
		sorted := r.selectProviders(r.providers, req.Model, r.strategy)
		assert.Equal(t, 2, len(sorted))
		// p1 (explicit) and p3 (all) should be included
		names := []string{sorted[0].Name(), sorted[1].Name()}
		assert.Contains(t, names, "p1")
		assert.Contains(t, names, "p3")
		assert.NotContains(t, names, "p2")
	})

	t.Run("Filter by overlapping model (gpt-4)", func(t *testing.T) {
		req := &api.ChatCompletionRequest{Model: "gpt-4"}
		sorted := r.selectProviders(r.providers, req.Model, r.strategy)
		assert.Equal(t, 3, len(sorted)) // all support gpt-4
	})

	t.Run("Filter by unique model (claude-3)", func(t *testing.T) {
		req := &api.ChatCompletionRequest{Model: "claude-3"}
		sorted := r.selectProviders(r.providers, req.Model, r.strategy)
		assert.Equal(t, 2, len(sorted))
		names := []string{sorted[0].Name(), sorted[1].Name()}
		assert.Contains(t, names, "p2")
		assert.Contains(t, names, "p3")
	})

	t.Run("No matching providers", func(t *testing.T) {
		req := &api.ChatCompletionRequest{Model: "unknown-model"}
		// Since p3 supports "all", it should still be there
		sorted := r.selectProviders(r.providers, req.Model, r.strategy)
		assert.Equal(t, 1, len(sorted))
		assert.Equal(t, "p3", sorted[0].Name())
	})
	
	t.Run("Strict rejection", func(t *testing.T) {
		// Router with only strict providers
		r2 := NewRouter([]*ProviderWithMetadata{p1, p2}, "round-robin", true, 3)
		req := &api.ChatCompletionRequest{Model: "unknown-model"}
		resp, err := r2.ChatCompletion(ctx, req)
		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no providers support model")
	})
}
