// Package metrics defines the Prometheus metric collectors used across the
// platform and exposes an HTTP handler for scraping.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus collectors for the platform.
type Metrics struct {
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge
	SearchQueriesTotal   *prometheus.CounterVec
	SearchLatency        *prometheus.HistogramVec
	SearchResultsCount   *prometheus.HistogramVec
	CacheHitsTotal       prometheus.Counter
	CacheMissesTotal     prometheus.Counter
	DocsIndexedTotal     prometheus.Counter
	IndexFlushesTotal    *prometheus.CounterVec
	ShardDocCount        *prometheus.GaugeVec
	ActiveShards         prometheus.Gauge
	CircuitBreakerState  *prometheus.GaugeVec
}

// New creates and registers all Prometheus metrics.
func New() *Metrics {
	m := &Metrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests by method, path, and status.",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency in seconds.",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{"method", "path"},
		),
		HTTPRequestsInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Number of HTTP requests currently being processed.",
			},
		),
		SearchQueriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "search_queries_total",
				Help: "Total search queries by result type (hit, miss, zero_result, error).",
			},
			[]string{"result_type"},
		),
		SearchLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "search_latency_seconds",
				Help:    "Search query latency in seconds.",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
			},
			[]string{"cache_status"},
		),
		SearchResultsCount: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "search_results_count",
				Help:    "Number of results returned per search query.",
				Buckets: []float64{0, 1, 5, 10, 25, 50, 100},
			},
			[]string{},
		),
		CacheHitsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "Total number of cache hits.",
			},
		),
		CacheMissesTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "Total number of cache misses.",
			},
		),
		DocsIndexedTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "docs_indexed_total",
				Help: "Total documents indexed.",
			},
		),
		IndexFlushesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "index_flushes_total",
				Help: "Total index flush operations by status.",
			},
			[]string{"status"},
		),
		ShardDocCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "shard_document_count",
				Help: "Number of documents per shard.",
			},
			[]string{"shard_id"},
		),
		ActiveShards: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "active_shards",
				Help: "Number of active index shards.",
			},
		),
		CircuitBreakerState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "circuit_breaker_state",
				Help: "Circuit breaker state (0=closed, 1=open, 2=half-open).",
			},
			[]string{"name"},
		),
	}

	prometheus.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.HTTPRequestsInFlight,
		m.SearchQueriesTotal,
		m.SearchLatency,
		m.SearchResultsCount,
		m.CacheHitsTotal,
		m.CacheMissesTotal,
		m.DocsIndexedTotal,
		m.IndexFlushesTotal,
		m.ShardDocCount,
		m.ActiveShards,
		m.CircuitBreakerState,
	)

	return m
}

// Handler returns the Prometheus scrape HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}
