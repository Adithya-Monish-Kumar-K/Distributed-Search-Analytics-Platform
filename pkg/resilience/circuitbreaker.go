// Package resilience provides fault-tolerance primitives: a circuit breaker,
// exponential-backoff retry, and a context-based timeout wrapper.
package resilience

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is in the Open state.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// State represents the current phase of a circuit breaker.
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig controls failure thresholds and recovery timing.
type CircuitBreakerConfig struct {
	FailureThreshold    int
	ResetTimeout        time.Duration
	HalfOpenMaxRequests int
}

func defaultCBConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:    5,
		ResetTimeout:        30 * time.Second,
		HalfOpenMaxRequests: 1,
	}
}

// CircuitBreaker tracks consecutive failures and trips open when the
// threshold is exceeded. After a cool-down period it transitions to
// half-open and allows a probe request.
type CircuitBreaker struct {
	name                string
	cfg                 CircuitBreakerConfig
	mu                  sync.Mutex
	state               State
	logger              *slog.Logger
	consecutiveFailures int
	lastFailureTime     time.Time
	halfOpenRequests    int
}

// NewCircuitBreaker creates a CircuitBreaker with the given config, filling
// in defaults for zero values.
func NewCircuitBreaker(name string, cfg CircuitBreakerConfig) *CircuitBreaker {
	defaults := defaultCBConfig()
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = defaults.FailureThreshold
	}
	if cfg.ResetTimeout <= 0 {
		cfg.ResetTimeout = defaults.ResetTimeout
	}
	if cfg.HalfOpenMaxRequests <= 0 {
		cfg.HalfOpenMaxRequests = defaults.HalfOpenMaxRequests
	}
	return &CircuitBreaker{
		name:   name,
		cfg:    cfg,
		state:  StateClosed,
		logger: slog.Default().With("component", "circuit-breaker", "name", name),
	}
}

// Execute runs fn if the circuit allows it, recording success or failure.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if err := cb.beforeRequest(); err != nil {
		return err
	}
	err := fn()
	cb.afterRequest(err)
	return err
}

// GetState returns the current State of the circuit breaker.
func (cb *CircuitBreaker) GetState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		return nil
	case StateOpen:
		if time.Since(cb.lastFailureTime) >= cb.cfg.ResetTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenRequests = 0
			cb.logger.Info("circuit transitioning to half-open",
				"after", cb.cfg.ResetTimeout,
			)
			return nil
		}
		return fmt.Errorf("%w: %s (retry after %v)", ErrCircuitOpen, cb.name, cb.cfg.ResetTimeout-time.Since(cb.lastFailureTime))
	case StateHalfOpen:
		if cb.halfOpenRequests >= cb.cfg.HalfOpenMaxRequests {
			return fmt.Errorf("%w: %s (half-open probe limit reached)", ErrCircuitOpen, cb.name)
		}
		cb.halfOpenRequests++
		return nil
	}
	return nil
}

func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if err == nil {
		cb.onSuccess()
		return
	}
	cb.onFailure()
}

func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateClosed:
		cb.consecutiveFailures = 0
	case StateHalfOpen:
		cb.state = StateClosed
		cb.consecutiveFailures = 0
		cb.halfOpenRequests = 0
		cb.logger.Info("circuit closed (recovered)")
	}
}

func (cb *CircuitBreaker) onFailure() {
	cb.lastFailureTime = time.Now()
	cb.consecutiveFailures++
	switch cb.state {
	case StateClosed:
		if cb.consecutiveFailures >= cb.cfg.FailureThreshold {
			cb.state = StateOpen
			cb.logger.Warn("circuit opened", "consecutive_failures", cb.consecutiveFailures, "threshold", cb.cfg.FailureThreshold)
		}
	case StateHalfOpen:
		cb.state = StateOpen
		cb.logger.Warn("circuit re-opened (half-open probe failed)")
	}
}

// Reset forces the circuit breaker back to the Closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.consecutiveFailures = 0
	cb.halfOpenRequests = 0
	cb.logger.Info("circuit manually reset")
}
