// Package middleware provides HTTP middleware for the API gateway including
// authentication, CORS, and rate limiting.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/auth/apikey"
)

type contextKey string

const apiKeyInfoKey contextKey = "api_key_info"

// Auth returns middleware that validates API keys from the request.
// Keys can be provided via Authorization: Bearer <key>, X-API-Key header,
// or the api_key query parameter. Health endpoints are exempt.
func Auth(validator *apikey.Validator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health endpoints.
			if strings.HasPrefix(r.URL.Path, "/health") {
				next.ServeHTTP(w, r)
				return
			}

			key := extractAPIKey(r)
			if key == "" {
				writeError(w, http.StatusUnauthorized, "missing api key")
				return
			}

			info, err := validator.Validate(r.Context(), key)
			if err != nil {
				switch err {
				case apikey.ErrInvalidKey:
					writeError(w, http.StatusUnauthorized, "invalid api key")
				case apikey.ErrExpiredKey:
					writeError(w, http.StatusUnauthorized, "expired api key")
				default:
					writeError(w, http.StatusInternalServerError, "authentication error")
				}
				return
			}

			ctx := context.WithValue(r.Context(), apiKeyInfoKey, info)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetKeyInfo retrieves the validated KeyInfo from the request context.
func GetKeyInfo(ctx context.Context) *apikey.KeyInfo {
	info, _ := ctx.Value(apiKeyInfoKey).(*apikey.KeyInfo)
	return info
}

// extractAPIKey reads the API key from the request in priority order:
// Authorization: Bearer header, X-API-Key header, api_key query parameter.
func extractAPIKey(r *http.Request) string {
	// 1. Authorization: Bearer <key>
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	// 2. X-API-Key header
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	// 3. Query parameter
	return r.URL.Query().Get("api_key")
}

// writeError writes a JSON error response to the client.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":"` + message + `"}`))
}
