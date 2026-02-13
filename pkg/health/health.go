package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Status string

const (
	StatusUp       Status = "up"
	StatusDown     Status = "down"
	StatusDegraded Status = "degraded"
)

type Check func(ctx context.Context) ComponentHealth

type ComponentHealth struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

type Report struct {
	Status     Status                     `json:"status"`
	Components map[string]ComponentHealth `json:"components"`
	Timestamp  string                     `json:"timestamp"`
}

type Checker struct {
	checks map[string]Check
	mu     sync.RWMutex
	logger *slog.Logger
}

func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]Check),
		logger: slog.Default().With("component", "health"),
	}
}

func (c *Checker) Register(name string, check Check) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

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

func (c *Checker) LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "alive",
		})
	}
}

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
