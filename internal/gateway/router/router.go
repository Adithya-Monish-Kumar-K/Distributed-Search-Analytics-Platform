// Package router wires up all API gateway routes and applies the middleware
// chain (RequestID → CORS → Auth → RateLimit).
package router

import (
	"net/http"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/auth/apikey"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/auth/ratelimit"
	gwhandler "github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/gateway/handler"
	gwmw "github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/gateway/middleware"
	pkgmw "github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/middleware"
)

// New builds the full gateway HTTP handler with all routes and middleware.
//
// Route table:
//
//	POST   /api/v1/documents          → ingestion service (proxy)
//	GET    /api/v1/documents           → list documents   (direct DB)
//	GET    /api/v1/documents/{id}      → get document     (direct DB)
//	GET    /api/v1/search              → search service   (proxy)
//	GET    /api/v1/analytics           → search service   (proxy)
//	GET    /api/v1/cache/stats         → search service   (proxy)
//	POST   /api/v1/cache/invalidate    → search service   (proxy)
//	POST   /api/v1/admin/keys          → create API key   (direct DB)
//	GET    /api/v1/admin/keys          → list API keys    (direct DB)
//	GET    /health                     → gateway health
//
// Middleware chain (outermost first):
//
//	RequestID → CORS → Auth → RateLimit → handler
func New(h *gwhandler.Handler, validator *apikey.Validator, limiter *ratelimit.Limiter) http.Handler {
	mux := http.NewServeMux()

	// Health (unauthenticated)
	mux.HandleFunc("GET /health", h.Health)

	// Document API
	mux.HandleFunc("POST /api/v1/documents", h.ProxyIngest)
	mux.HandleFunc("GET /api/v1/documents", h.ListDocuments)
	mux.HandleFunc("GET /api/v1/documents/{id}", h.GetDocument)

	// Search API
	mux.HandleFunc("GET /api/v1/search", h.ProxySearch)

	// Analytics API
	mux.HandleFunc("GET /api/v1/analytics", h.ProxyAnalytics)

	// Cache API
	mux.HandleFunc("GET /api/v1/cache/stats", h.ProxyCacheStats)
	mux.HandleFunc("POST /api/v1/cache/invalidate", h.ProxyCacheInvalidate)

	// Admin API
	mux.HandleFunc("POST /api/v1/admin/keys", h.CreateAPIKey)
	mux.HandleFunc("GET /api/v1/admin/keys", h.ListAPIKeys)

	// Middleware chain — applied inside-out:
	// request → RequestID → CORS → Auth → RateLimit → mux
	var chain http.Handler = mux
	chain = gwmw.RateLimit(limiter)(chain)
	chain = gwmw.Auth(validator)(chain)
	chain = gwmw.CORS(gwmw.DefaultCORSConfig())(chain)
	chain = pkgmw.RequestID(chain)

	return chain
}
