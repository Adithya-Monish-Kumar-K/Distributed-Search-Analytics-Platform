// Package logger configures the global slog logger and provides helpers to
// propagate request-scoped fields (such as request IDs) through context.
package logger

import (
	"context"
	"log/slog"
	"os"
)

type contextKey struct{}

// Setup configures the global slog logger with the given level and format
// ("json" or "text").
func Setup(level string, format string) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: parseLevel(level),
	}
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// WithRequestID stores a request ID in the context for later retrieval by
// FromContext.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, contextKey{}, requestID)
}

// FromContext returns a logger enriched with the request ID from ctx, if
// present.
func FromContext(ctx context.Context) *slog.Logger {
	logger := slog.Default()
	if requestID, ok := ctx.Value(contextKey{}).(string); ok {
		logger = logger.With("request_id", requestID)
	}
	return logger
}

// WithComponent returns a logger with the "component" field set.
func WithComponent(component string) *slog.Logger {
	return slog.Default().With("component", component)
}

// parseLevel converts a level string to an slog.Level.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
