package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			done := make(chan struct{})
			tw := &timeoutWriter{ResponseWriter: w}
			go func() {
				next.ServeHTTP(tw, r.WithContext(ctx))
				close(done)
			}()
			select {
			case <-done:
			case <-ctx.Done():
				if !tw.written {
					slog.Warn("request timed out", "method", r.Method, "path", r.URL.Path, "timeout", timeout)
					http.Error(w, `{"error":"request timeout"}`, http.StatusGatewayTimeout)
				}
			}
		})
	}
}

type timeoutWriter struct {
	http.ResponseWriter
	written bool
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.written = true
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.written = true
	return tw.ResponseWriter.Write(b)
}
