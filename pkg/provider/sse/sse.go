package sse

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/user/llmrouter/pkg/api"
)

// Stream reads an OpenAI-compatible SSE response body and emits parsed stream chunks.
// It owns and closes the body when done.
func Stream(ctx context.Context, body io.ReadCloser) (<-chan *api.ChatCompletionStreamResponse, <-chan error) {
	respCh := make(chan *api.ChatCompletionStreamResponse)
	errCh := make(chan error, 1)

	go func() {
		defer close(respCh)
		defer close(errCh)
		defer body.Close()

		scanner := bufio.NewScanner(body)
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

			var chunk api.ChatCompletionStreamResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// skip malformed chunks
				continue
			}

			select {
			case respCh <- &chunk:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			errCh <- err
		}
	}()

	return respCh, errCh
}
