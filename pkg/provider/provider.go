// Package provider defines the interface for LLM providers.
package provider

import (
	"context"
	"github.com/user/llmrouter/pkg/api"
)

// Provider defines the interface for all LLM backends.
type Provider interface {
	// Name returns the unique name of the provider.
	Name() string
	// ChatCompletion sends a request to the provider and returns the response.
	ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error)
	// StreamChatCompletion sends a request to the provider and returns a channel of streaming responses.
	StreamChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (<-chan *api.ChatCompletionStreamResponse, <-chan error)
}
