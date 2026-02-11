package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/executor"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/parser"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/ranker"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/logger"
)

// Handler handles HTTP requests for the search API.
type Handler struct {
	executor     *executor.Executor
	defaultLimit int
	maxResults   int
	logger       *slog.Logger
}

// New creates a search Handler.
func New(exec *executor.Executor, defaultLimit int, maxResults int) *Handler {
	return &Handler{
		executor:     exec,
		defaultLimit: defaultLimit,
		maxResults:   maxResults,
		logger:       slog.Default().With("component", "search-handler"),
	}
}

// Search handles GET /api/v1/search?q=...&limit=...
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)

	// Parse query parameters
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

	// Parse the query
	plan := parser.Parse(query)
	if len(plan.Terms) == 0 {
		h.writeJSON(w, http.StatusOK, &executor.SearchResult{
			Query:   query,
			Results: []ranker.ScoredDoc{},
		})
		return
	}

	// Execute
	result, err := h.executor.Execute(ctx, plan, limit)
	if err != nil {
		log.Error("search execution failed",
			"query", query,
			"error", err,
		)
		h.writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	log.Info("search completed",
		"query", query,
		"total_hits", result.TotalHits,
		"returned", len(result.Results),
	)

	h.writeJSON(w, http.StatusOK, result)
}

// Health handles GET /health
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
