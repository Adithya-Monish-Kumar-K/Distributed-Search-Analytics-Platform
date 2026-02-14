// Package health provides a concurrent health-check framework. Components
// register Check functions, and the Checker runs them in parallel to produce
// an aggregate Report suitable for Kubernetes liveness and readiness probes.
package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Status represents the health state of a component or the system overall.
type Status string

const (
	StatusUp       Status = "up"
	StatusDown     Status = "down"
	StatusDegraded Status = "degraded"
)

// Check is a function that probes a single dependency and returns its status.
type Check func(ctx context.Context) ComponentHealth

// ComponentHealth holds the result of a single component check.
type ComponentHealth struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Report is the aggregated result of all component checks.
type Report struct {
	Status     Status                     `json:"status"`
	Components map[string]ComponentHealth `json:"components"`
	Timestamp  string                     `json:"timestamp"`
}

// Checker manages registered health checks and runs them concurrently.
type Checker struct {
	checks map[string]Check
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewChecker creates an empty Checker.
func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]Check),
		logger: slog.Default().With("component", "health"),
	}
}

// Register adds a named health check.
func (c *Checker) Register(name string, check Check) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

// Run executes all registered checks concurrently and returns an aggregated
// Report. The overall status is the worst status among all components.
func (c *Checker) Run(ctx context.Context) Report {
	c.mu.RLock()
	checks := make(map[string]Check, len(c.checks))
	for name, check := range c.checks {
		checks[name] = check
	}
	c.mu.RUnlock()
	report := Report{
		Status:     StatusUp,
		Components: make(map[string]ComponentHealth, len(checks)),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, check := range checks {
		wg.Add(1)
		go func(n string, ch Check) {
			defer wg.Done()
			start := time.Now()
			result := ch(ctx)
			result.Latency = time.Since(start).Round(time.Millisecond).String()
			mu.Lock()
			report.Components[n] = result
			mu.Unlock()
		}(name, check)
	}
	wg.Wait()
	for _, comp := range report.Components {
		switch comp.Status {
		case StatusDown:
			report.Status = StatusDown
			return report
		case StatusDegraded:
			report.Status = StatusDegraded
		}
	}
	return report
}

// LiveHandler returns an HTTP handler for Kubernetes liveness probes.
func (c *Checker) LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "alive",
		})
	}
}

// ReadyHandler returns an HTTP handler for Kubernetes readiness probes.
func (c *Checker) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		report := c.Run(ctx)
		w.Header().Set("Content-Type", "application/json")
		if report.Status == StatusUp {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(report)
	}
}
