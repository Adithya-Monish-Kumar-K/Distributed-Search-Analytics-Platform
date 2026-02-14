// Package handler exposes the search service HTTP endpoints including query
// execution, cache management, and health checks.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/analytics"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/cache"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/executor"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/parser"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/ranker"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/logger"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/metrics"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/middleware"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/tracing"
)

// SearchExecutor abstracts single-shard and sharded query execution.
type SearchExecutor interface {
	Execute(ctx context.Context, plan *parser.QueryPlan, limit int) (*executor.SearchResult, error)
}

// Handler serves the search service HTTP API.
type Handler struct {
	executor     SearchExecutor
	cache        *cache.QueryCache
	collector    *analytics.Collector
	metrics      *metrics.Metrics
	defaultLimit int
	maxResults   int
	logger       *slog.Logger
}

// New creates a Handler with the given executor, cache, analytics collector,
// metrics recorder, and result-limit settings.
func New(exec SearchExecutor, queryCache *cache.QueryCache, collector *analytics.Collector, m *metrics.Metrics, defaultLimit, maxResults int) *Handler {
	return &Handler{
		executor:     exec,
		cache:        queryCache,
		collector:    collector,
		metrics:      m,
		defaultLimit: defaultLimit,
		maxResults:   maxResults,
		logger:       slog.Default().With("component", "search-handler"),
	}
}

// Search handles GET /api/v1/search?q=&limit=. It parses the query,
// optionally checks the cache, executes the plan, records metrics and
// analytics, and writes the JSON result.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()
	log := logger.FromContext(ctx)

	requestID := middleware.GetRequestID(ctx)
	ctx, span := tracing.StartSpan(ctx, "search", requestID)
	defer func() {
		span.End()
		span.Log()
	}()

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

	_, parseSpan := tracing.StartChildSpan(ctx, "parse_query")
	plan := parser.Parse(query)
	parseSpan.SetAttr("terms", len(plan.Terms))
	parseSpan.SetAttr("exclude_terms", len(plan.ExcludeTerms))
	parseSpan.End()

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
		_, cacheSpan := tracing.StartChildSpan(ctx, "cache_lookup")
		result, cacheHit, err = h.cache.GetOrCompute(ctx, query, limit, func() (*executor.SearchResult, error) {
			_, execSpan := tracing.StartChildSpan(ctx, "execute_query")
			defer execSpan.End()
			return h.executor.Execute(ctx, plan, limit)
		})
		cacheSpan.SetAttr("hit", cacheHit)
		cacheSpan.End()
	} else {
		_, execSpan := tracing.StartChildSpan(ctx, "execute_query")
		result, err = h.executor.Execute(ctx, plan, limit)
		execSpan.End()
	}

	if err != nil {
		log.Error("search execution failed", "query", query, "error", err)
		h.recordSearchMetrics("error", false, 0, time.Since(start))
		h.writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	latencyMs := time.Since(start).Milliseconds()
	duration := time.Since(start)

	resultType := "hit"
	if result.TotalHits == 0 {
		resultType = "zero_result"
	}

	h.recordSearchMetrics(resultType, cacheHit, len(result.Results), duration)

	span.SetAttr("query", query)
	span.SetAttr("total_hits", result.TotalHits)
	span.SetAttr("returned", len(result.Results))
	span.SetAttr("cache_hit", cacheHit)
	span.SetAttr("latency_ms", latencyMs)

	log.Info("search completed",
		"query", query,
		"total_hits", result.TotalHits,
		"returned", len(result.Results),
		"cache_hit", cacheHit,
		"latency_ms", latencyMs,
	)

	if h.collector != nil {
		eventType := analytics.EventCacheMiss
		if cacheHit {
			eventType = analytics.EventCacheHit
		}

		h.collector.Track(analytics.SearchEvent{
			Type:      eventType,
			Query:     query,
			Terms:     plan.Terms,
			TotalHits: result.TotalHits,
			Returned:  len(result.Results),
			LatencyMs: latencyMs,
			CacheHit:  cacheHit,
			Timestamp: time.Now().UTC(),
			RequestID: requestID,
		})
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"query":     result.Query,
		"total":     result.TotalHits,
		"results":   result.Results,
		"took_ms":   float64(latencyMs),
		"cache_hit": cacheHit,
	})
}

// recordSearchMetrics updates Prometheus counters and histograms for the
// completed search.
func (h *Handler) recordSearchMetrics(resultType string, cacheHit bool, resultCount int, duration time.Duration) {
	if h.metrics == nil {
		return
	}

	h.metrics.SearchQueriesTotal.WithLabelValues(resultType).Inc()

	cacheStatus := "miss"
	if cacheHit {
		cacheStatus = "hit"
		h.metrics.CacheHitsTotal.Inc()
	} else {
		h.metrics.CacheMissesTotal.Inc()
	}

	h.metrics.SearchLatency.WithLabelValues(cacheStatus).Observe(duration.Seconds())
	h.metrics.SearchResultsCount.WithLabelValues().Observe(float64(resultCount))
}

// CacheStats returns current cache hit/miss counts and hit rate.
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

// CacheInvalidate flushes all cached search results.
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

// Health returns a simple health-check response.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeJSON serialises data as JSON and writes it with the given status code.
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to write response", "error", err)
	}
}

// writeError writes a JSON error response.
func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
