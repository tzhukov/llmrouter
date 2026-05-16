package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/user/llmrouter/pkg/api"
)

// MockProvider is a provider used for testing and simulation.
type MockProvider struct {
	name    string
	latency time.Duration
	err     error
}

func NewMockProvider(name string, latency time.Duration, err error) *MockProvider {
	return &MockProvider{
		name:    name,
		latency: latency,
		err:     err,
	}
}

func (p *MockProvider) Name() string {
	return p.name
}

func (p *MockProvider) ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	if p.latency > 0 {
		select {
		case <-time.After(p.latency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if p.err != nil {
		return nil, p.err
	}

	return &api.ChatCompletionResponse{
		ID:      fmt.Sprintf("mock-%s-%d", p.name, time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []api.ChatCompletionChoice{
			{
				Index: 0,
				Message: api.ChatCompletionMessage{
					Role:    "assistant",
					Content: fmt.Sprintf("This is a mock response from %s", p.name),
				},
				FinishReason: "stop",
			},
		},
		Usage: api.ChatCompletionUsage{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
		},
	}, nil
}

func (p *MockProvider) StreamChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (<-chan *api.ChatCompletionStreamResponse, <-chan error) {
	respCh := make(chan *api.ChatCompletionStreamResponse)
	errCh := make(chan error, 1)

	go func() {
		defer close(respCh)
		defer close(errCh)

		if p.latency > 0 {
			select {
			case <-time.After(p.latency):
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}

		if p.err != nil {
			errCh <- p.err
			return
		}

		id := fmt.Sprintf("mock-stream-%s-%d", p.name, time.Now().Unix())
		words := []string{"This ", "is ", "a ", "mock ", "streaming ", "response ", "from ", p.name}

		for i, word := range words {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case <-time.After(50 * time.Millisecond): // Simulate token generation delay
				respCh <- &api.ChatCompletionStreamResponse{
					ID:      id,
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   req.Model,
					Choices: []api.ChatCompletionStreamChoice{
						{
							Index: 0,
							Delta: api.ChatCompletionStreamDelta{
								Content: word,
							},
						},
					},
				}
				if i == len(words)-1 {
					// Send final chunk with finish reason
					respCh <- &api.ChatCompletionStreamResponse{
						ID:      id,
						Object:  "chat.completion.chunk",
						Created: time.Now().Unix(),
						Model:   req.Model,
						Choices: []api.ChatCompletionStreamChoice{
							{
								Index:        0,
								Delta:        api.ChatCompletionStreamDelta{},
								FinishReason: "stop",
							},
						},
					}
				}
			}
		}
	}()

	return respCh, errCh
}
