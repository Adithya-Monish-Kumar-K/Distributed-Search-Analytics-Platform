package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/cache"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/executor"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/parser"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/ranker"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/logger"
)

type SearchExecutor interface {
	Execute(ctx context.Context, plan *parser.QueryPlan, limit int) (*executor.SearchResult, error)
}
type Handler struct {
	executor     SearchExecutor
	cache        *cache.QueryCache
	defaultLimit int
	maxResults   int
	logger       *slog.Logger
}

func New(exec SearchExecutor, queryCache *cache.QueryCache, defaultLimit, maxResults int) *Handler {
	return &Handler{
		executor:     exec,
		cache:        queryCache,
		defaultLimit: defaultLimit,
		maxResults:   maxResults,
		logger:       slog.Default().With("component", "search-handler"),
	}
}
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	query := r.URL.Query().Get("q")
	if query == "" {
		h.writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit := h.defaultLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed < 1 {
			h.writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > h.maxResults {
			parsed = h.maxResults
		}
		limit = parsed
	}

	plan := parser.Parse(query)
	if len(plan.Terms) == 0 {
		h.writeJSON(w, http.StatusOK, &executor.SearchResult{
			Query:   query,
			Results: []ranker.ScoredDoc{},
		})
		return
	}

	var result *executor.SearchResult
	var err error
	cacheHit := false

	if h.cache != nil {
		result, cacheHit, err = h.cache.GetOrCompute(ctx, query, limit, func() (*executor.SearchResult, error) {
			return h.executor.Execute(ctx, plan, limit)
		})
	} else {
		result, err = h.executor.Execute(ctx, plan, limit)
	}

	if err != nil {
		log.Error("search execution failed", "query", query, "error", err)
		h.writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	log.Info("search completed",
		"query", query,
		"total_hits", result.TotalHits,
		"returned", len(result.Results),
		"cache_hit", cacheHit,
	)

	h.writeJSON(w, http.StatusOK, result)
}
func (h *Handler) CacheStats(w http.ResponseWriter, r *http.Request) {
	if h.cache == nil {
		h.writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
		return
	}

	hits, misses := h.cache.Stats()
	total := hits + misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"hits":     hits,
		"misses":   misses,
		"total":    total,
		"hit_rate": fmt.Sprintf("%.1f%%", hitRate),
	})
}
func (h *Handler) CacheInvalidate(w http.ResponseWriter, r *http.Request) {
	if h.cache == nil {
		h.writeError(w, http.StatusServiceUnavailable, "caching is disabled")
		return
	}

	if err := h.cache.Invalidate(r.Context()); err != nil {
		h.logger.Error("cache invalidation failed", "error", err)
		h.writeError(w, http.StatusInternalServerError, "cache invalidation failed")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "invalidated"})
}
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to write response", "error", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
