package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/user/llmrouter/pkg/api"
)

func TestAgentRoutingIntegration(t *testing.T) {
	s := NewServer()
	
	// Create a temporary config file with two agents
	configContent := `
agents:
  default:
    routing:
      strategy: "round-robin"
    providers:
      - name: default-mock
        type: mock
  coder:
    routing:
      strategy: "latency"
    providers:
      - name: coder-mock
        type: mock
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())
	
	_, err = tmpfile.Write([]byte(configContent))
	assert.NoError(t, err)
	tmpfile.Close()

	// Load the config
	err = s.reloadConfig(tmpfile.Name())
	assert.NoError(t, err)

	t.Run("Default Agent Routing", func(t *testing.T) {
		req := api.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []api.ChatCompletionMessage{
				{Role: "user", Content: "hello"},
			},
		}
		body, _ := json.Marshal(req)
		r := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		w := httptest.NewRecorder()

		s.handleChatCompletion(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp api.ChatCompletionResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
	})

	t.Run("Specific Agent Routing", func(t *testing.T) {
		req := api.ChatCompletionRequest{
			AgentID: "coder",
			Model: "gpt-4",
			Messages: []api.ChatCompletionMessage{
				{Role: "user", Content: "hello"},
			},
		}
		body, _ := json.Marshal(req)
		r := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		w := httptest.NewRecorder()

		s.handleChatCompletion(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Unknown Agent Fallback", func(t *testing.T) {
		req := api.ChatCompletionRequest{
			AgentID: "unknown",
			Model: "gpt-4",
			Messages: []api.ChatCompletionMessage{
				{Role: "user", Content: "hello"},
			},
		}
		body, _ := json.Marshal(req)
		r := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		w := httptest.NewRecorder()

		s.handleChatCompletion(w, r)

		assert.Equal(t, http.StatusOK, w.Code, "Should fallback to default agent")
	})

	t.Run("Hot Reload Agents", func(t *testing.T) {
		// Add a new agent "researcher"
		newConfig := configContent + `
  researcher:
    routing:
      strategy: "cost"
    providers:
      - name: researcher-mock
        type: mock
`
		err = os.WriteFile(tmpfile.Name(), []byte(newConfig), 0644)
		assert.NoError(t, err)

		// Trigger reload
		err = s.reloadConfig(tmpfile.Name())
		assert.NoError(t, err)

		req := api.ChatCompletionRequest{
			AgentID: "researcher",
			Model: "gpt-4",
			Messages: []api.ChatCompletionMessage{
				{Role: "user", Content: "hello"},
			},
		}
		body, _ := json.Marshal(req)
		r := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
		w := httptest.NewRecorder()

		s.handleChatCompletion(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
