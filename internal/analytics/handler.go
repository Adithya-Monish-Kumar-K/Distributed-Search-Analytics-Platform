package analytics

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Handler struct {
	aggregator *Aggregator
	logger     *slog.Logger
}

func NewHandler(aggregator *Aggregator) *Handler {
	return &Handler{
		aggregator: aggregator,
		logger:     slog.Default().With("component", "analytics-handler"),
	}
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats := h.aggregator.Stats()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		h.logger.Error("failed to write analytics response", "error", err)
	}
}
