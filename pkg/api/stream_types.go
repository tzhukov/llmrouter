// Package api defines the data structures for the llmrouter API.
package api

// ChatCompletionStreamResponse represents a single chunk of a streaming chat completion response.
type ChatCompletionStreamResponse struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"`
	Created int64                        `json:"created"`
	Model   string                       `json:"model"`
	Choices []ChatCompletionStreamChoice `json:"choices"`
}

// ChatCompletionStreamChoice represents a single choice in a streaming response chunk.
type ChatCompletionStreamChoice struct {
	Index        int                       `json:"index"`
	Delta        ChatCompletionStreamDelta `json:"delta"`
	FinishReason string                    `json:"finish_reason,omitempty"`
}

// ChatCompletionStreamDelta represents the incremental content in a streaming response chunk.
type ChatCompletionStreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}
