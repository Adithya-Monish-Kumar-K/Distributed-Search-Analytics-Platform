package handler

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/auth/apikey"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/postgres"
)

// Config holds the URLs of backend services that the gateway proxies to.
type Config struct {
	IngestionURL string
	SearcherURL  string
}

// Handler implements the API gateway's HTTP endpoints.
// It proxies requests to backend services and provides direct
// document retrieval and API key management via PostgreSQL.
type Handler struct {
	ingestionProxy *httputil.ReverseProxy
	searchProxy    *httputil.ReverseProxy
	db             *postgres.Client
	keyValidator   *apikey.Validator
	logger         *slog.Logger
}

// New creates a gateway Handler that proxies to the given backend URLs.
func New(cfg Config, db *postgres.Client, keyValidator *apikey.Validator) *Handler {
	return &Handler{
		ingestionProxy: newProxy(cfg.IngestionURL),
		searchProxy:    newProxy(cfg.SearcherURL),
		db:             db,
		keyValidator:   keyValidator,
		logger:         slog.Default().With("component", "gateway-handler"),
	}
}

func newProxy(target string) *httputil.ReverseProxy {
	u, _ := url.Parse(target)
	return httputil.NewSingleHostReverseProxy(u)
}

// ---------- Proxy handlers ----------

// ProxyIngest forwards document ingestion requests to the ingestion service.
func (h *Handler) ProxyIngest(w http.ResponseWriter, r *http.Request) {
	h.ingestionProxy.ServeHTTP(w, r)
}

// ProxySearch forwards search queries to the search service.
func (h *Handler) ProxySearch(w http.ResponseWriter, r *http.Request) {
	h.searchProxy.ServeHTTP(w, r)
}

// ProxyAnalytics forwards analytics requests to the search service.
func (h *Handler) ProxyAnalytics(w http.ResponseWriter, r *http.Request) {
	h.searchProxy.ServeHTTP(w, r)
}

// ProxyCacheStats forwards cache stats requests to the search service.
func (h *Handler) ProxyCacheStats(w http.ResponseWriter, r *http.Request) {
	h.searchProxy.ServeHTTP(w, r)
}

// ProxyCacheInvalidate forwards cache invalidation requests to the search service.
func (h *Handler) ProxyCacheInvalidate(w http.ResponseWriter, r *http.Request) {
	h.searchProxy.ServeHTTP(w, r)
}

// ---------- Direct data handlers ----------

// GetDocument retrieves a single document's metadata from PostgreSQL by UUID.
func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "document id is required")
		return
	}

	var doc struct {
		ID          string     `json:"id"`
		Title       string     `json:"title"`
		ContentHash string     `json:"content_hash"`
		ContentSize int        `json:"content_size"`
		ShardID     int        `json:"shard_id"`
		Status      string     `json:"status"`
		CreatedAt   time.Time  `json:"created_at"`
		IndexedAt   *time.Time `json:"indexed_at,omitempty"`
	}

	err := h.db.DB.QueryRowContext(r.Context(),
		`SELECT id, title, content_hash, content_size, shard_id, status, created_at, indexed_at
		 FROM documents WHERE id = $1`, id,
	).Scan(&doc.ID, &doc.Title, &doc.ContentHash, &doc.ContentSize,
		&doc.ShardID, &doc.Status, &doc.CreatedAt, &doc.IndexedAt)

	if err == sql.ErrNoRows {
		h.writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		h.logger.Error("failed to fetch document", "id", id, "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to fetch document")
		return
	}

	h.writeJSON(w, http.StatusOK, doc)
}

// ListDocuments returns a paginated list of document metadata.
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	rows, err := h.db.DB.QueryContext(r.Context(),
		`SELECT id, title, shard_id, status, created_at
		 FROM documents ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		h.logger.Error("failed to list documents", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to list documents")
		return
	}
	defer rows.Close()

	type docSummary struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		ShardID   int       `json:"shard_id"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
	}

	docs := make([]docSummary, 0)
	for rows.Next() {
		var d docSummary
		if err := rows.Scan(&d.ID, &d.Title, &d.ShardID, &d.Status, &d.CreatedAt); err != nil {
			h.logger.Error("failed to scan document row", "error", err)
			continue
		}
		docs = append(docs, d)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"documents": docs,
		"count":     len(docs),
		"limit":     limit,
		"offset":    offset,
	})
}

// ---------- Admin handlers ----------

// CreateAPIKey creates a new API key and returns the raw key (shown once).
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		RateLimit int    `json:"rate_limit"`
		ExpiresIn string `json:"expires_in,omitempty"` // Go duration, e.g. "720h"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.RateLimit <= 0 {
		req.RateLimit = 100
	}

	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		d, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid expires_in duration")
			return
		}
		t := time.Now().Add(d)
		expiresAt = &t
	}

	key, err := h.keyValidator.CreateKey(r.Context(), req.Name, req.RateLimit, expiresAt)
	if err != nil {
		h.logger.Error("failed to create api key", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to create api key")
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]string{
		"api_key": key,
		"name":    req.Name,
		"message": "store this key securely — it cannot be retrieved again",
	})
}

// ListAPIKeys returns all active API keys (without hashes).
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.keyValidator.ListKeys(r.Context())
	if err != nil {
		h.logger.Error("failed to list api keys", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to list api keys")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"keys":  keys,
		"count": len(keys),
	})
}

// ---------- Health ----------

// Health returns the gateway's health status.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "gateway"})
}

// ---------- Helpers ----------

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
