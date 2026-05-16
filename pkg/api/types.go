package api

// ChatCompletionRequest represents a request to the chat completion endpoint.
type ChatCompletionRequest struct {
	AgentID          string                `json:"agent_id,omitempty"`
	Model            string                `json:"model"`
	Messages         []ChatCompletionMessage `json:"messages"`
	Stream           bool                  `json:"stream,omitempty"`
	Temperature      *float32              `json:"temperature,omitempty"`
	TopP             *float32              `json:"top_p,omitempty"`
	MaxTokens        *int                  `json:"max_tokens,omitempty"`
	PresencePenalty  *float32              `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float32              `json:"frequency_penalty,omitempty"`
	User             string                `json:"user,omitempty"`
}

// ChatCompletionMessage represents a single message in a chat completion request/response.
type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// ChatCompletionResponse represents a response from the chat completion endpoint.
type ChatCompletionResponse struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"`
	Created int64                        `json:"created"`
	Model   string                       `json:"model"`
	Choices []ChatCompletionChoice       `json:"choices"`
	Usage   ChatCompletionUsage          `json:"usage"`
}

// ChatCompletionChoice represents a single choice in a chat completion response.
type ChatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

// ChatCompletionUsage represents the token usage for a chat completion request.
type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
