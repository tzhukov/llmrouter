package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"github.com/user/llmrouter/pkg/api"
	"github.com/user/llmrouter/pkg/config"
	"github.com/user/llmrouter/pkg/observability"
	"github.com/user/llmrouter/pkg/router"
)

// Server represents the LLM router server.
type Server struct {
	Router   *chi.Mux
	Registry *router.Registry
}

// NewServer creates a new Server instance.
func NewServer() *Server {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(observability.LoggingMiddleware)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	s := &Server{
		Router:   r,
		Registry: router.NewRegistry(),
	}

	s.routes()

	return s
}

func (s *Server) routes() {
	s.Router.Get("/health", s.handleHealth)
	s.Router.Handle("/metrics", promhttp.Handler())

	s.Router.Route("/v1", func(r chi.Router) {
		r.Post("/chat/completions", s.handleChatCompletion)
		r.Post("/responses", s.handleResponses)

		// Debug route to catch what LiteLLM is sending
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			log.Warn().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Interface("headers", r.Header).
				Msg("unimplemented endpoint hit")
			http.Error(w, fmt.Sprintf("endpoint %s not implemented", r.URL.Path), http.StatusNotFound)
		})
	})
}

type responsesInputItem struct {
	Role    string                `json:"role"`
	Content []responsesInputChunk `json:"content"`
}

type responsesInputChunk struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responsesRequest struct {
	Model  string               `json:"model"`
	Input  []responsesInputItem `json:"input"`
	Stream bool                 `json:"stream"`
}

type responsesOutputChunk struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responsesOutputItem struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Status  string                 `json:"status"`
	Role    string                 `json:"role"`
	Content []responsesOutputChunk `json:"content"`
}

type responsesResponse struct {
	ID         string                `json:"id"`
	Object     string                `json:"object"`
	CreatedAt  int64                 `json:"created_at"`
	Model      string                `json:"model"`
	Status     string                `json:"status"`
	Output     []responsesOutputItem `json:"output"`
	OutputText string                `json:"output_text"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Error().Err(err).Msg("failed to write health response")
	}
}

func (s *Server) handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	var req api.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("failed to decode request")
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	engine := s.Registry.GetRouter(req.AgentID)
	if engine == nil {
		log.Error().Str("agent_id", req.AgentID).Msg("no router found for agent")
		http.Error(w, "no router available for this agent", http.StatusServiceUnavailable)
		return
	}

	if req.Stream {
		s.handleChatCompletionStream(w, r, &req, engine)
		return
	}

	resp, err := engine.ChatCompletion(r.Context(), &req)
	if err != nil {
		log.Error().Err(err).Msg("routing failed")
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("failed to encode response")
	}
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	var req responsesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("failed to decode responses request")
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Stream {
		// Attempt to handle as chat completion stream if possible
		messages := s.translateResponsesToMessages(req.Input)
		chatReq := &api.ChatCompletionRequest{
			Model:    req.Model,
			Messages: messages,
			Stream:   true,
		}

		engine := s.Registry.GetRouter("")
		if engine == nil {
			http.Error(w, "no router available", http.StatusServiceUnavailable)
			return
		}

		// This is a hack: we are sending ChatCompletion stream back to a Responses client.
		// Most clients (like Claude Code via LiteLLM) might not like this if they expect the Responses SSE format.
		// However, returning 501 definitely breaks it.
		log.Warn().Msg("handling streaming responses request as chat completion (compatibility hack)")
		s.handleChatCompletionStream(w, r, chatReq, engine)
		return
	}

	messages := s.translateResponsesToMessages(req.Input)
	chatReq := &api.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
	}

	engine := s.Registry.GetRouter("")
	if engine == nil {
		log.Error().Msg("no router found")
		http.Error(w, "no router available", http.StatusServiceUnavailable)
		return
	}

	chatResp, err := engine.ChatCompletion(r.Context(), chatReq)
	if err != nil {
		log.Error().Err(err).Msg("responses routing failed")
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	text := ""
	if len(chatResp.Choices) > 0 {
		text = chatResp.Choices[0].Message.Content
	}

	resp := responsesResponse{
		ID:        chatResp.ID,
		Object:    "response",
		CreatedAt: chatResp.Created,
		Model:     chatResp.Model,
		Status:    "completed",
		Output: []responsesOutputItem{{
			ID:     chatResp.ID + "-msg-0",
			Type:   "message",
			Status: "completed",
			Role:   "assistant",
			Content: []responsesOutputChunk{{
				Type: "output_text",
				Text: text,
			}},
		}},
		OutputText: text,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("failed to encode response")
	}
}

func (s *Server) translateResponsesToMessages(input []responsesInputItem) []api.ChatCompletionMessage {
	messages := make([]api.ChatCompletionMessage, 0, len(input))
	for _, item := range input {
		parts := make([]string, 0, len(item.Content))
		for _, c := range item.Content {
			if c.Text != "" {
				parts = append(parts, c.Text)
			}
		}
		if len(parts) == 0 {
			continue
		}
		messages = append(messages, api.ChatCompletionMessage{
			Role:    item.Role,
			Content: strings.Join(parts, "\n"),
		})
	}
	return messages
}

func (s *Server) handleChatCompletionStream(w http.ResponseWriter, r *http.Request, req *api.ChatCompletionRequest, engine *router.Router) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, canFlush := w.(http.Flusher)
	flush := func() {
		if canFlush {
			flusher.Flush()
		}
	}

	respCh, errCh := engine.StreamChatCompletion(r.Context(), req)

	// Use nil-ing pattern to avoid infinite loops on closed channels.
	activeCh := respCh
	activeErrCh := errCh
	for activeCh != nil || activeErrCh != nil {
		select {
		case chunk, ok := <-activeCh:
			if !ok {
				activeCh = nil
				continue
			}
			data, _ := json.Marshal(chunk)
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				log.Error().Err(err).Msg("failed to write stream data")
				return
			}
			flush()
		case err, ok := <-activeErrCh:
			if !ok {
				activeErrCh = nil
				continue
			}
			if err != nil {
				log.Error().Err(err).Msg("streaming failed")
				if _, err := fmt.Fprintf(w, "data: {\"error\":\"%s\"}\n\n", err.Error()); err != nil {
					log.Error().Err(err).Msg("failed to write stream error")
				}
				flush()
				return
			}
		case <-r.Context().Done():
			return
		}
	}

	// All chunks delivered — send OpenAI-compatible stream terminator.
	if _, err := fmt.Fprint(w, "data: [DONE]\n\n"); err != nil {
		log.Error().Err(err).Msg("failed to write stream terminator")
	}
	flush()
}

// WatchRemoteConfig starts watching configuration from a remote URL.
func (s *Server) WatchRemoteConfig(ctx context.Context, url string) {
	remoteProvider := config.NewRemoteProvider(url)
	updateCh := make(chan map[string]config.AgentConfig)

	go remoteProvider.Watch(ctx, updateCh)

	go func() {
		for {
			select {
			case agents := <-updateCh:
				s.Registry.UpdateAgents(agents)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Start starts the server on the given address.
func (s *Server) Start(addr string) error {
	log.Info().Str("addr", addr).Msg("starting server")
	return http.ListenAndServe(addr, s.Router)
}
