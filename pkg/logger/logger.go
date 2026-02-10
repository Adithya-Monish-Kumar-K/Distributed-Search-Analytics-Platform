package logger

import (
	"context"
	"log/slog"
	"os"
)

type contextKey struct{}

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

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, contextKey{}, requestID)
}

func FromContext(ctx context.Context) *slog.Logger {
	logger := slog.Default()
	if requestID, ok := ctx.Value(contextKey{}).(string); ok {
		logger = logger.With("request_id", requestID)
	}
	return logger
}

func WithComponent(component string) *slog.Logger {
	return slog.Default().With("component", component)
}

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
