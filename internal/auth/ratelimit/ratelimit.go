package ratelimit

import (
	"sync"
	"time"
)

// entry tracks the token-bucket state for a single key.
type entry struct {
	tokens    float64
	lastCheck time.Time
}

// Limiter implements an in-memory token-bucket rate limiter.
// Tokens refill at a rate of (limit / window) per second.
type Limiter struct {
	mu      sync.Mutex
	entries map[string]*entry
	window  time.Duration
}

// New creates a rate limiter with the given refill window.
// Each key gets `limit` tokens per window, refilled continuously.
func New(window time.Duration) *Limiter {
	l := &Limiter{
		entries: make(map[string]*entry),
		window:  window,
	}
	go l.cleanup()
	return l
}

// Allow checks whether the given key has remaining capacity.
// It consumes one token on success and returns true.
// Returns false when the rate limit has been exceeded.
func (l *Limiter) Allow(key string, limit int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	e, exists := l.entries[key]
	if !exists {
		l.entries[key] = &entry{
			tokens:    float64(limit - 1),
			lastCheck: now,
		}
		return true
	}

	elapsed := now.Sub(e.lastCheck)
	e.lastCheck = now

	// Refill tokens proportionally to elapsed time.
	rate := float64(limit) / l.window.Seconds()
	e.tokens += elapsed.Seconds() * rate
	if e.tokens > float64(limit) {
		e.tokens = float64(limit)
	}

	if e.tokens < 1 {
		return false
	}

	e.tokens--
	return true
}

// Reset clears the rate-limit state for a specific key.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// cleanup periodically removes stale entries to prevent memory leaks.
func (l *Limiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		cutoff := time.Now().Add(-2 * l.window)
		for key, e := range l.entries {
			if e.lastCheck.Before(cutoff) {
				delete(l.entries, key)
			}
		}
		l.mu.Unlock()
	}
}
