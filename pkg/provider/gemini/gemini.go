package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/user/llmrouter/pkg/api"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
const defaultModel = "gemini-1.5-flash"

// GeminiProvider adapts the Google Gemini API to the provider.Provider interface.
type GeminiProvider struct {
	name       string
	apiKey     string
	baseURL    string
	httpClient *http.Client
	modelMap   map[string]string
}

func NewGeminiProvider(name, apiKey, baseURL string) *GeminiProvider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if name == "" {
		name = "gemini"
	}
	return &GeminiProvider{
		name:       name,
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{},
		modelMap:   make(map[string]string),
	}
}

func (p *GeminiProvider) SetModelMap(m map[string]string) {
	p.modelMap = m
}

func (p *GeminiProvider) Name() string {
	return p.name
}

// ... (other types unchanged)

func (p *GeminiProvider) modelName(req *api.ChatCompletionRequest) string {
	if req.Model == "" {
		return defaultModel
	}
	if mapped, ok := p.modelMap[req.Model]; ok {
		return mapped
	}
	return req.Model
}

// --- Gemini native types ---

type geminiRequest struct {
	Contents         []geminiContent         `json:"contents"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature     *float32 `json:"temperature,omitempty"`
	TopP            *float32 `json:"topP,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate   `json:"candidates"`
	UsageMetadata geminiUsageMetadata `json:"usageMetadata"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
	Index        int           `json:"index"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// --- Translation helpers ---

// toGeminiRole maps OpenAI roles to Gemini roles.
// "assistant" → "model", everything else (including "system") → "user".
func toGeminiRole(role string) string {
	if role == "assistant" {
		return "model"
	}
	return "user"
}

func toGeminiRequest(req *api.ChatCompletionRequest) *geminiRequest {
	contents := make([]geminiContent, 0, len(req.Messages))
	for _, m := range req.Messages {
		contents = append(contents, geminiContent{
			Role:  toGeminiRole(m.Role),
			Parts: []geminiPart{{Text: m.Content}},
		})
	}

	gr := &geminiRequest{Contents: contents}

	if req.Temperature != nil || req.TopP != nil || req.MaxTokens != nil {
		gr.GenerationConfig = &geminiGenerationConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
		}
	}

	return gr
}

// --- ChatCompletion ---

func (p *GeminiProvider) ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	actualModel := p.modelName(req)
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, actualModel, p.apiKey)

	body, err := json.Marshal(toGeminiRequest(req))
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini api error: status code %d", resp.StatusCode)
	}

	var gr geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, err
	}

	return toOpenAIResponse(&gr, actualModel), nil
}

func toOpenAIResponse(gr *geminiResponse, model string) *api.ChatCompletionResponse {
	choices := make([]api.ChatCompletionChoice, 0, len(gr.Candidates))
	for _, c := range gr.Candidates {
		text := ""
		if len(c.Content.Parts) > 0 {
			text = c.Content.Parts[0].Text
		}
		choices = append(choices, api.ChatCompletionChoice{
			Index: c.Index,
			Message: api.ChatCompletionMessage{
				Role:    "assistant",
				Content: text,
			},
			FinishReason: strings.ToLower(c.FinishReason),
		})
	}

	return &api.ChatCompletionResponse{
		ID:      fmt.Sprintf("gemini-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: choices,
		Usage: api.ChatCompletionUsage{
			PromptTokens:     gr.UsageMetadata.PromptTokenCount,
			CompletionTokens: gr.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      gr.UsageMetadata.TotalTokenCount,
		},
	}
}

// --- StreamChatCompletion ---

func (p *GeminiProvider) StreamChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (<-chan *api.ChatCompletionStreamResponse, <-chan error) {
	respCh := make(chan *api.ChatCompletionStreamResponse)
	errCh := make(chan error, 1)

	errOut := func(err error) (<-chan *api.ChatCompletionStreamResponse, <-chan error) {
		close(respCh)
		errCh <- err
		close(errCh)
		return respCh, errCh
	}

	actualModel := p.modelName(req)
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s&alt=sse", p.baseURL, actualModel, p.apiKey)

	body, err := json.Marshal(toGeminiRequest(req))
	if err != nil {
		return errOut(err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return errOut(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return errOut(err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return errOut(fmt.Errorf("gemini api error: status code %d", resp.StatusCode))
	}

	go func() {
		defer close(respCh)
		defer close(errCh)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var gr geminiResponse
			if err := json.Unmarshal([]byte(data), &gr); err != nil {
				// skip malformed chunks
				continue
			}

			chunk := toOpenAIStreamChunk(&gr, actualModel)
			select {
			case respCh <- chunk:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}()

	return respCh, errCh
}

func toOpenAIStreamChunk(gr *geminiResponse, model string) *api.ChatCompletionStreamResponse {
	choices := make([]api.ChatCompletionStreamChoice, 0, len(gr.Candidates))
	for _, c := range gr.Candidates {
		text := ""
		if len(c.Content.Parts) > 0 {
			text = c.Content.Parts[0].Text
		}
		choices = append(choices, api.ChatCompletionStreamChoice{
			Index: c.Index,
			Delta: api.ChatCompletionStreamDelta{
				Role:    "assistant",
				Content: text,
			},
			FinishReason: strings.ToLower(c.FinishReason),
		})
	}

	return &api.ChatCompletionStreamResponse{
		ID:      fmt.Sprintf("gemini-%d", time.Now().UnixNano()),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: choices,
	}
}
