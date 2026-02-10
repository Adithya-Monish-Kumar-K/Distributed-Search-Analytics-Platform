package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Adithya-Monish-Kumar-K/searchplatform/internal/ingestion"
	"github.com/Adithya-Monish-Kumar-K/searchplatform/internal/ingestion/publisher"
	"github.com/Adithya-Monish-Kumar-K/searchplatform/internal/ingestion/validator"
	apperrors "github.com/Adithya-Monish-Kumar-K/searchplatform/pkg/errors"
	"github.com/Adithya-Monish-Kumar-K/searchplatform/pkg/logger"
)

type Handler struct {
	publisher *publisher.Publisher
	logger    *slog.Logger
}

func New(pub *publisher.Publisher) *Handler {
	return &Handler{
		publisher: pub,
		logger:    slog.Default().With("component", "ingestion-handler"),
	}
}

func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.FromContext(ctx)
	var req ingestion.IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := validator.ValidateIngestRequest(&req); err != nil {
		var validationErr *validator.ValidationError
		if errors.As(err, &validationErr) {
			h.writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":  "validation failed",
				"fields": validationErr.Fields,
			})
			return
		}
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.publisher.Ingest(ctx, &req)
	if err != nil {
		statusCode := apperrors.HTTPStatusCode(err)
		log.Error("ingestion failed",
			"error", err,
			"status_code", statusCode,
		)
		h.writeError(w, statusCode, "ingestion failed")
		return
	}
	log.Info("document ingested",
		"doc_id", resp.DocumentID,
		"shard_id", resp.ShardID,
	)
	h.writeJSON(w, http.StatusAccepted, resp)
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
