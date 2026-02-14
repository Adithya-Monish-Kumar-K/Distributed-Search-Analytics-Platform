// Package middleware provides reusable HTTP middleware for request IDs,
// Prometheus metrics, and request timeouts.
package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/metrics"
)

// Metrics returns middleware that records HTTP request count, latency, and
// in-flight gauge.
func Metrics(m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			m.HTTPRequestsInFlight.Inc()
			defer m.HTTPRequestsInFlight.Dec()

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			duration := time.Since(start).Seconds()
			path := normalizePath(r.URL.Path)

			m.HTTPRequestsTotal.WithLabelValues(
				r.Method,
				path,
				strconv.Itoa(sw.status),
			).Inc()

			m.HTTPRequestDuration.WithLabelValues(
				r.Method,
				path,
			).Observe(duration)
		})
	}
}

// statusWriter wraps http.ResponseWriter to capture the response status code.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.wroteHeader {
		sw.status = code
		sw.wroteHeader = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.wroteHeader {
		sw.wroteHeader = true
	}
	return sw.ResponseWriter.Write(b)
}

// normalizePath returns the path as-is; can be extended to collapse
// path parameters.
func normalizePath(path string) string {
	return path
}
