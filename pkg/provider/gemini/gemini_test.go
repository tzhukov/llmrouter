package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/user/llmrouter/pkg/api"
)

func makeRequest() *api.ChatCompletionRequest {
	return &api.ChatCompletionRequest{
		Model: "gemini-1.5-flash",
		Messages: []api.ChatCompletionMessage{
			{Role: "user", Content: "Hello"},
		},
	}
}

func geminiResponseBody(text string) geminiResponse {
	return geminiResponse{
		Candidates: []geminiCandidate{
			{
				Index: 0,
				Content: geminiContent{
					Role:  "model",
					Parts: []geminiPart{{Text: text}},
				},
				FinishReason: "STOP",
			},
		},
		UsageMetadata: geminiUsageMetadata{
			PromptTokenCount:     5,
			CandidatesTokenCount: 10,
			TotalTokenCount:      15,
		},
	}
}

func TestChatCompletion_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Query().Get("key") != "test-key" {
			t.Errorf("expected api key in query, got %q", r.URL.Query().Get("key"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(geminiResponseBody("Hi there!"))
	}))
	defer srv.Close()

	p := NewGeminiProvider("gemini-test", "test-key", srv.URL)
	resp, err := p.ChatCompletion(context.Background(), makeRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hi there!" {
		t.Errorf("unexpected content: %q", resp.Choices[0].Message.Content)
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("expected role assistant, got %q", resp.Choices[0].Message.Role)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("expected 15 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestChatCompletion_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := NewGeminiProvider("gemini-test", "bad-key", srv.URL)
	_, err := p.ChatCompletion(context.Background(), makeRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestChatCompletion_RoleTranslation(t *testing.T) {
	var captured geminiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(geminiResponseBody("ok"))
	}))
	defer srv.Close()

	p := NewGeminiProvider("gemini-test", "key", srv.URL)
	req := &api.ChatCompletionRequest{
		Model: "gemini-1.5-flash",
		Messages: []api.ChatCompletionMessage{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
		},
	}
	p.ChatCompletion(context.Background(), req)

	roles := []string{"user", "user", "model"} // system → user, assistant → model
	for i, c := range captured.Contents {
		if c.Role != roles[i] {
			t.Errorf("contents[%d]: expected role %q, got %q", i, roles[i], c.Role)
		}
	}
}

func TestStreamChatCompletion_Success(t *testing.T) {
	chunks := []geminiResponse{
		{Candidates: []geminiCandidate{{Index: 0, Content: geminiContent{Role: "model", Parts: []geminiPart{{Text: "Hello"}}}}}},
		{Candidates: []geminiCandidate{{Index: 0, Content: geminiContent{Role: "model", Parts: []geminiPart{{Text: " world"}}}, FinishReason: "STOP"}}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, c := range chunks {
			b, _ := json.Marshal(c)
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
	}))
	defer srv.Close()

	p := NewGeminiProvider("gemini-test", "key", srv.URL)
	respCh, errCh := p.StreamChatCompletion(context.Background(), makeRequest())

	var texts []string
	for chunk := range respCh {
		if len(chunk.Choices) > 0 {
			texts = append(texts, chunk.Choices[0].Delta.Content)
		}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}

	if len(texts) != 2 || texts[0] != "Hello" || texts[1] != " world" {
		t.Errorf("unexpected stream content: %v", texts)
	}
}

func TestStreamChatCompletion_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	p := NewGeminiProvider("gemini-test", "bad-key", srv.URL)
	respCh, errCh := p.StreamChatCompletion(context.Background(), makeRequest())

	for range respCh {
	}
	if err := <-errCh; err == nil {
		t.Fatal("expected error, got nil")
	}
}
