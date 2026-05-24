// Package groq implements the Groq provider for the llmrouter.
package groq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/user/llmrouter/pkg/api"
	"github.com/user/llmrouter/pkg/provider/sse"
)

// Provider implements the Groq LLM provider.
type Provider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewProvider creates a new Groq provider.
func NewProvider(apiKey string, baseURL string) *Provider {
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	return &Provider{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "groq"
}

// ChatCompletion sends a chat completion request to Groq.
func (p *Provider) ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	url := fmt.Sprintf("%s/chat/completions", p.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("groq api error: status code %d", resp.StatusCode)
	}

	var completionResp api.ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
		return nil, err
	}

	return &completionResp, nil
}

// StreamChatCompletion sends a streaming chat completion request to Groq.
func (p *Provider) StreamChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (<-chan *api.ChatCompletionStreamResponse, <-chan error) {
	errOut := func(err error) (<-chan *api.ChatCompletionStreamResponse, <-chan error) {
		ch := make(chan *api.ChatCompletionStreamResponse)
		ec := make(chan error, 1)
		ec <- err
		close(ch)
		close(ec)
		return ch, ec
	}

	streamReq := *req
	streamReq.Stream = true

	url := fmt.Sprintf("%s/chat/completions", p.baseURL)
	body, err := json.Marshal(streamReq)
	if err != nil {
		return errOut(err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return errOut(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return errOut(err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return errOut(fmt.Errorf("groq api error: status code %d", resp.StatusCode))
	}

	return sse.Stream(ctx, resp.Body)
}
