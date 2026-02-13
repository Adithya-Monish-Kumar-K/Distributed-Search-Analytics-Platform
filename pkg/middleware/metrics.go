package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/metrics"
)

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
func normalizePath(path string) string {
	return path
}
