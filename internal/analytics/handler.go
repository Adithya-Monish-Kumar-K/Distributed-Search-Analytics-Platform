package analytics

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Handler exposes an HTTP endpoint for reading aggregated analytics.
type Handler struct {
	aggregator *Aggregator
	logger     *slog.Logger
}

// NewHandler creates a Handler backed by the given Aggregator.
func NewHandler(aggregator *Aggregator) *Handler {
	return &Handler{
		aggregator: aggregator,
		logger:     slog.Default().With("component", "analytics-handler"),
	}
}

// Stats handles GET /api/v1/analytics and returns the current aggregated stats
// as JSON.
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats := h.aggregator.Stats()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		h.logger.Error("failed to write analytics response", "error", err)
	}
}
