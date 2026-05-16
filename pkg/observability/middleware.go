package observability

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LoggingMiddleware adds the request ID to the logger context.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetReqID(r.Context())
		
		// Create a logger with request ID
		logger := log.With().Str("request_id", requestID).Logger()
		
		// Put logger in context
		ctx := logger.WithContext(r.Context())
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetLogger returns the logger from the context or the global logger if not found.
func GetLogger(ctx context.Context) *zerolog.Logger {
	return log.Ctx(ctx)
}
