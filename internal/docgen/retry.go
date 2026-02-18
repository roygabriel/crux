package docgen

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"time"
)

// Retryer executes a function with exponential backoff and jitter.
type Retryer struct {
	// MaxAttempts is the total number of attempts (including the first).
	MaxAttempts int
	// BaseDelay is the initial delay before the first retry.
	BaseDelay time.Duration
	// MaxDelay caps the delay between retries.
	MaxDelay time.Duration
	// Jitter is the fraction of the delay to randomise (0.0–1.0).
	Jitter float64
	// Logger records retry attempts.
	Logger *slog.Logger
	// RetryableCheck determines if an error is retryable. If nil, all errors
	// are retried.
	RetryableCheck func(error) bool
}

// DefaultRetryer returns a Retryer with sensible defaults: 5 attempts,
// 1s base delay, 60s max delay, and 0.25 jitter.
func DefaultRetryer(logger *slog.Logger) *Retryer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Retryer{
		MaxAttempts: 5,
		BaseDelay:   1 * time.Second,
		MaxDelay:    60 * time.Second,
		Jitter:      0.25,
		Logger:      logger,
	}
}

// Do executes fn, retrying on error up to MaxAttempts times. It respects
// context cancellation between retries.
func (r *Retryer) Do(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := range r.MaxAttempts {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if r.RetryableCheck != nil && !r.RetryableCheck(lastErr) {
			return lastErr
		}

		if attempt == r.MaxAttempts-1 {
			break
		}

		d := r.delay(attempt)
		r.Logger.Warn("retrying after error",
			"attempt", attempt+1,
			"max_attempts", r.MaxAttempts,
			"delay", d,
			"error", lastErr,
		)

		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(d):
		}
	}

	return fmt.Errorf("all %d attempts exhausted: %w", r.MaxAttempts, lastErr)
}

// delay computes the backoff duration for a given attempt with jitter.
func (r *Retryer) delay(attempt int) time.Duration {
	d := float64(r.BaseDelay) * math.Pow(2, float64(attempt))
	if d > float64(r.MaxDelay) {
		d = float64(r.MaxDelay)
	}

	if r.Jitter > 0 {
		jitterRange := d * r.Jitter
		d = d - jitterRange + rand.Float64()*2*jitterRange
	}

	return time.Duration(d)
}

// IsRetryableHTTPStatus reports whether an HTTP status code indicates a
// transient error that warrants a retry.
func IsRetryableHTTPStatus(status int) bool {
	switch status {
	case 429, 500, 502, 503, 529:
		return true
	default:
		return false
	}
}
