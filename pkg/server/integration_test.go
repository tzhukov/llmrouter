package server_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/llmrouter/pkg/api"
	"github.com/user/llmrouter/pkg/server"
)

func TestIntegration_RoutingAndHotReload(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "llmrouter-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	
	// 1. Initial Config: 2 Mock Providers
	initialConfig := `
providers:
  - name: mock-1
    type: mock
  - name: mock-2
    type: mock
`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatal(err)
	}

	s := server.NewServer()
	if err := s.WatchConfig(configPath); err != nil {
		t.Fatal(err)
	}

	// 2. Verify Round Robin
	reqBody, _ := json.Marshal(api.ChatCompletionRequest{Model: "test"})
	
	// Request 1 -> mock-1 or mock-2
	resp1 := doRequest(s, reqBody)
	// Request 2 -> the other one
	resp2 := doRequest(s, reqBody)

	if resp1.ID == resp2.ID {
		t.Errorf("expected different IDs for round-robin, got both %s", resp1.ID)
	}

	// 3. Hot Reload: Change to 1 Provider
	updatedConfig := `
providers:
  - name: updated-mock
    type: mock
`
	if err := os.WriteFile(configPath, []byte(updatedConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for watcher to pick up changes
	time.Sleep(500 * time.Millisecond)

	resp3 := doRequest(s, reqBody)
	expectedPrefix := "mock-updated-mock"
	if len(resp3.ID) < len(expectedPrefix) || resp3.ID[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected ID to start with %s, got %s", expectedPrefix, resp3.ID)
	}
}

func doRequest(s *server.Server, body []byte) api.ChatCompletionResponse {
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	s.Router.ServeHTTP(w, req)

	var resp api.ChatCompletionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp
}

func TestIntegration_Streaming(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "llmrouter-stream-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	config := `
providers:
  - name: mock-stream
    type: mock
routing:
  strategy: round-robin
`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	s := server.NewServer()
	if err := s.WatchConfig(configPath); err != nil {
		t.Fatal(err)
	}

	reqBody, _ := json.Marshal(api.ChatCompletionRequest{
		Model:  "test-model",
		Stream: true,
		Messages: []api.ChatCompletionMessage{
			{Role: "user", Content: "hello"},
		},
	})

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBuffer(reqBody))
	w := httptest.NewRecorder()
	s.Router.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify Content-Type is SSE
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}

	// Parse SSE lines and count chunks
	var chunks []api.ChatCompletionStreamResponse
	var doneSeen bool
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			doneSeen = true
			continue
		}
		var chunk api.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			t.Errorf("failed to parse SSE chunk %q: %v", data, err)
			continue
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		t.Error("expected at least one stream chunk, got none")
	}
	if !doneSeen {
		t.Error("expected data: [DONE] terminator, not found")
	}

	// Verify chunks have content
	var totalContent string
	for _, c := range chunks {
		if len(c.Choices) > 0 {
			totalContent += c.Choices[0].Delta.Content
		}
	}
	if totalContent == "" {
		t.Error("expected non-empty streamed content")
	}
	t.Logf("streamed %d chunks, content: %q, [DONE]=%v", len(chunks), totalContent, doneSeen)
}
