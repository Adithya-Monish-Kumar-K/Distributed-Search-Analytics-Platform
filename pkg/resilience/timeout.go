package resilience

import (
	"context"
	"fmt"
	"time"
)

// WithTimeout runs fn with a derived context that is cancelled after the
// given timeout. If the function does not complete in time,
// context.DeadlineExceeded is returned.
func WithTimeout(ctx context.Context, timeout time.Duration, name string, fn func(ctx context.Context) error) error {
	if timeout <= 0 {
		return fn(ctx)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- fn(timeoutCtx)
	}()
	select {
	case err := <-done:
		return err
	case <-timeoutCtx.Done():
		if ctx.Err() != nil {
			return fmt.Errorf("%s: parent context cancelled: %w", name, ctx.Err())
		}
		return fmt.Errorf("%s: %w (limit: %v)", name, context.DeadlineExceeded, timeout)
	}
}
