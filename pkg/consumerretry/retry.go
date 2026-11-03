// Package consumerretry provides timeout and exponential-backoff retry for Kafka record processing.
package consumerretry

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const (
	AttemptTimeout = 10 * time.Second
	InitialBackoff = 500 * time.Millisecond
	MaxBackoff     = 5 * time.Second
	MaxAttempts    = 6
)

// Backoff returns an exponentially increasing delay capped at MaxBackoff.
func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := InitialBackoff
	for current := 1; current < attempt && delay < MaxBackoff; current++ {
		delay *= 2
		if delay > MaxBackoff {
			delay = MaxBackoff
		}
	}
	return delay
}

// Do retries fn with an attempt timeout and increasing delays. After MaxAttempts, the caller
// should stop the consumer so Kubernetes can restart it and Kafka can redeliver the uncommitted record.
func Do(ctx context.Context, logger *slog.Logger, operation string, fn func(context.Context) error) error {
	var last error
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, AttemptTimeout)
		last = fn(attemptCtx)
		cancel()
		if last == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt == MaxAttempts {
			break
		}

		wait := Backoff(attempt)
		logger.Warn("event operation failed; retrying",
			slog.String("operation", operation),
			slog.Int("attempt", attempt),
			slog.Int64("retry_in_ms", wait.Milliseconds()),
			slog.Any("error", last),
		)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("%s failed after %d attempts: %w", operation, MaxAttempts, last)
}
