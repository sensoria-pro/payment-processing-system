package observability

import (
	"context"
	"log/slog"
	"net/http"
	"os"
)

type contextKey string

const loggerKey = contextKey("logger")

// SetupLogger define a global logger (default is slog.)
func SetupLogger(env string) *slog.Logger {
	var logger *slog.Logger
	switch env {
	case  "development", "dev":
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	default:
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return logger
}

// NewLoggerMiddleware Adds a logger to the context of each request.
func NewLoggerMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), loggerKey, logger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
