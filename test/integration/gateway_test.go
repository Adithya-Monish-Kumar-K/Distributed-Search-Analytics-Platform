// Package integration contains tests that verify the interaction between
// multiple platform components. These tests use httptest servers with real
// handler wiring but mock external dependencies (Kafka, PostgreSQL, Redis).
//
// Run with:
//
//	go test -v -tags=integration ./test/integration/...
package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/auth/apikey"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/auth/ratelimit"
	gwhandler "github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/gateway/handler"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/gateway/router"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/postgres"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// skipIfNoPostgres skips the test when PostgreSQL is unavailable.
func skipIfNoPostgres(t *testing.T) *postgres.Client {
	t.Helper()
	cfg := testPostgresConfig()
	db, err := postgres.New(cfg)
	if err != nil {
		t.Skipf("skipping integration test: postgres unavailable: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testPostgresConfig() config.PostgresConfig {
	return config.PostgresConfig{
		Host:            envOrDefault("TEST_POSTGRES_HOST", "localhost"),
		Port:            envOrDefaultInt("TEST_POSTGRES_PORT", 5432),
		Database:        envOrDefault("TEST_POSTGRES_DB", "searchplatform_test"),
		User:            envOrDefault("TEST_POSTGRES_USER", "searchplatform"),
		Password:        envOrDefault("TEST_POSTGRES_PASSWORD", "localdev"),
		SSLMode:         "disable",
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

// newGatewayServer creates a test gateway backed by a real PostgreSQL database.
func newGatewayServer(t *testing.T, db *postgres.Client) *httptest.Server {
	t.Helper()

	// Dummy backend services — return 200 for proxied requests.
	ingestionBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]any{
			"document_id": "00000000-0000-0000-0000-000000000001",
			"status":      "PENDING",
			"shard_id":    0,
		})
	}))
	t.Cleanup(ingestionBackend.Close)

	searchBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"query":      r.URL.Query().Get("q"),
			"total_hits": 0,
			"results":    []any{},
		})
	}))
	t.Cleanup(searchBackend.Close)

	validator := apikey.NewValidator(db)
	limiter := ratelimit.New(60_000_000_000) // 1 minute window

	h := gwhandler.New(gwhandler.Config{
		IngestionURL: ingestionBackend.URL,
		SearcherURL:  searchBackend.URL,
	}, db, validator)

	chain := router.New(h, validator, limiter)
	return httptest.NewServer(chain)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestHealthEndpoint verifies the gateway health check is accessible without auth.
func TestHealthEndpoint(t *testing.T) {
	db := skipIfNoPostgres(t)
	srv := newGatewayServer(t, db)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", body["status"])
	}
}

// TestUnauthenticatedRequestRejected verifies that API endpoints reject
// requests without an API key.
func TestUnauthenticatedRequestRejected(t *testing.T) {
	db := skipIfNoPostgres(t)
	srv := newGatewayServer(t, db)
	defer srv.Close()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/search?q=test"},
		{"GET", "/api/v1/documents"},
		{"GET", "/api/v1/analytics"},
	}

	for _, ep := range endpoints {
		req, _ := http.NewRequest(ep.method, srv.URL+ep.path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: request failed: %v", ep.method, ep.path, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401, got %d", ep.method, ep.path, resp.StatusCode)
		}
	}
}

// TestAPIKeyLifecycle tests creating, using, and revoking an API key
// through the gateway when PostgreSQL is available.
func TestAPIKeyLifecycle(t *testing.T) {
	db := skipIfNoPostgres(t)
	srv := newGatewayServer(t, db)
	defer srv.Close()

	// For this test we bypass the gateway auth and use the validator directly
	// since the admin endpoints also require auth (chicken-and-egg).
	validator := apikey.NewValidator(db)

	// 1. Create a key directly.
	rawKey, err := validator.CreateKey(t.Context(), "integration-test", 100, nil)
	if err != nil {
		t.Fatalf("creating key: %v", err)
	}

	// 2. Use the key to hit the search endpoint.
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/search?q=hello", nil)
	req.Header.Set("X-API-Key", rawKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("search request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	// 3. Revoke the key.
	if err := validator.RevokeKey(t.Context(), rawKey); err != nil {
		t.Fatalf("revoking key: %v", err)
	}

	// 4. Verify the revoked key is rejected.
	req2, _ := http.NewRequest("GET", srv.URL+"/api/v1/search?q=hello", nil)
	req2.Header.Set("X-API-Key", rawKey)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("search request after revoke failed: %v", err)
	}
	resp2.Body.Close()

	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 after revoke, got %d", resp2.StatusCode)
	}
}

// TestDocumentIngestProxy verifies that document ingestion is proxied through
// the gateway to the ingestion backend.
func TestDocumentIngestProxy(t *testing.T) {
	db := skipIfNoPostgres(t)
	srv := newGatewayServer(t, db)
	defer srv.Close()

	validator := apikey.NewValidator(db)
	rawKey, err := validator.CreateKey(t.Context(), "ingest-test", 100, nil)
	if err != nil {
		t.Fatalf("creating key: %v", err)
	}

	payload := map[string]string{
		"title": "Test Document",
		"body":  "This is a test document for integration testing.",
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/documents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", rawKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("ingest request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, respBody)
	}
}

// TestRateLimiting verifies that the gateway enforces per-key rate limits.
func TestRateLimiting(t *testing.T) {
	db := skipIfNoPostgres(t)
	srv := newGatewayServer(t, db)
	defer srv.Close()

	validator := apikey.NewValidator(db)
	// Create a key with a very low rate limit.
	rawKey, err := validator.CreateKey(t.Context(), "ratelimit-test", 2, nil)
	if err != nil {
		t.Fatalf("creating key: %v", err)
	}

	// First 2 requests should succeed.
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", srv.URL+"/api/v1/search?q=test", nil)
		req.Header.Set("X-API-Key", rawKey)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, resp.StatusCode)
		}
	}

	// 3rd request should be rate limited.
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/search?q=test", nil)
	req.Header.Set("X-API-Key", rawKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("rate limit request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Env helpers
// ---------------------------------------------------------------------------

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
