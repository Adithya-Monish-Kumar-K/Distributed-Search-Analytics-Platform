package middleware

import (
	"net/http"
	"strings"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/auth/ratelimit"
)

// RateLimit returns middleware that enforces per-key rate limits.
// It reads the KeyInfo from context (set by Auth middleware) and uses
// the key's configured rate_limit value. Requests without a key are
// passed through (let Auth middleware reject them instead).
func RateLimit(limiter *ratelimit.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for health endpoints.
			if strings.HasPrefix(r.URL.Path, "/health") {
				next.ServeHTTP(w, r)
				return
			}

			info := GetKeyInfo(r.Context())
			if info == nil {
				// No key info in context — let the request through
				// (Auth middleware will block unauthenticated requests).
				next.ServeHTTP(w, r)
				return
			}

			if !limiter.Allow(info.ID, info.RateLimit) {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
