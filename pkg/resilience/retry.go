package resilience

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"
)

type RetryConfig struct {
	MaxAttempts    int
	InitialDelay   time.Duration
	MaxDelay       time.Duration
	Multiplier     float64
	JitterFraction float64
}

func defaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:    3,
		InitialDelay:   100 * time.Millisecond,
		MaxDelay:       10 * time.Second,
		Multiplier:     2.0,
		JitterFraction: 0.1,
	}
}

func Retry(ctx context.Context, name string, cfg RetryConfig, fn func() error) error {
	defaults := defaultRetryConfig()
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = defaults.MaxAttempts
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = defaults.InitialDelay
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = defaults.MaxDelay
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = defaults.Multiplier
	}
	if cfg.JitterFraction <= 0 {
		cfg.JitterFraction = defaults.JitterFraction
	}
	logger := slog.Default().With("component", "retry", "operation", name)
	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			if attempt > 1 {
				logger.Info("succeeded after retry", "attempt", attempt)
			}
			return nil
		}
		if attempt == cfg.MaxAttempts {
			break
		}
		if ctx.Err() != nil {
			return fmt.Errorf("retry aborted: %w", ctx.Err())
		}
		delay := computeDelay(attempt, cfg)
		logger.Warn("operation failed, retrying", "attempt", attempt, "max_attempts", cfg.MaxAttempts, "error", lastErr, "next_delay", delay)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return fmt.Errorf("retry aborted during backoff: %w", ctx.Err())
		}
	}
	return fmt.Errorf("all %d attempts failed for %s: %w", cfg.MaxAttempts, name, lastErr)
}

func computeDelay(attempt int, cfg RetryConfig) time.Duration {
	backoff := float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))
	jitter := backoff * cfg.JitterFraction * (2*rand.Float64() - 1)
	backoff += jitter
	if backoff > float64(cfg.MaxDelay) {
		backoff = float64(cfg.MaxDelay)
	}
	if backoff < 0 {
		backoff = float64(cfg.InitialDelay)
	}
	return time.Duration(backoff)
}
