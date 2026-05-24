package provider_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/user/llmrouter/pkg/api"
	"github.com/user/llmrouter/pkg/provider/mock"
)

func TestMockProvider(t *testing.T) {
	t.Run("Successful response", func(t *testing.T) {
		p := mock.NewProvider("test-mock", 0, nil)
		req := &api.ChatCompletionRequest{
			Model: "test-model",
			Messages: []api.ChatCompletionMessage{
				{Role: "user", Content: "hi"},
			},
		}

		resp, err := p.ChatCompletion(context.Background(), req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.Model != "test-model" {
			t.Errorf("expected model test-model, got %s", resp.Model)
		}

		if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
			t.Errorf("expected content in response")
		}
	})

	t.Run("Error response", func(t *testing.T) {
		expectedErr := errors.New("provider failure")
		p := mock.NewProvider("test-mock", 0, expectedErr)

		_, err := p.ChatCompletion(context.Background(), &api.ChatCompletionRequest{})
		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("Latency and Timeout", func(t *testing.T) {
		p := mock.NewProvider("test-mock", 100*time.Millisecond, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := p.ChatCompletion(ctx, &api.ChatCompletionRequest{})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected deadline exceeded error, got %v", err)
		}
	})
}
