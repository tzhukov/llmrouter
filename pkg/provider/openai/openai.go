package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/user/llmrouter/pkg/api"
	"github.com/user/llmrouter/pkg/provider/sse"
)

type OpenAIProvider struct {
	apiKey     string
	baseUrl    string
	httpClient *http.Client
}

func NewOpenAIProvider(apiKey string, baseUrl string) *OpenAIProvider {
	if baseUrl == "" {
		baseUrl = "https://api.openai.com/v1"
	}
	return &OpenAIProvider{
		apiKey:     apiKey,
		baseUrl:    baseUrl,
		httpClient: &http.Client{},
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	url := fmt.Sprintf("%s/chat/completions", p.baseUrl)
	
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai api error: status code %d", resp.StatusCode)
	}

	var completionResp api.ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
		return nil, err
	}

	return &completionResp, nil
}

func (p *OpenAIProvider) StreamChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (<-chan *api.ChatCompletionStreamResponse, <-chan error) {
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

	url := fmt.Sprintf("%s/chat/completions", p.baseUrl)
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
		resp.Body.Close()
		return errOut(fmt.Errorf("openai api error: status code %d", resp.StatusCode))
	}

	return sse.Stream(ctx, resp.Body)
}
