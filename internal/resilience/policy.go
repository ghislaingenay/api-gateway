// Package resilience provides retry-policy and deadline primitives shared by
// outbound downstream calls (FEAT-008): exponential backoff with jitter for
// transient failures, and context-deadline derivation.
package resilience

import (
	"context"
	"math/rand"
	"time"
)

// RetryPolicy controls retry attempts for idempotent downstream calls.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts (including the first),
	// not the number of retries.
	MaxAttempts int
	// BaseBackoff is the base delay used to compute exponential backoff
	// between attempts.
	BaseBackoff time.Duration
}

// retryableStatus is the set of downstream response statuses considered
// transient and eligible for retry (FEAT-008 FR-1).
var retryableStatus = map[int]bool{
	502: true,
	503: true,
	504: true,
}

// IsRetryableStatus reports whether status is a transient failure eligible
// for retry.
func IsRetryableStatus(status int) bool {
	return retryableStatus[status]
}

// Backoff returns the delay before the given retry attempt (1-indexed),
// using exponential backoff with full jitter: a random duration in
// [0, BaseBackoff*2^(attempt-1)].
func (p RetryPolicy) Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	max := p.BaseBackoff << (attempt - 1)
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(max)))
}

// WithDeadline derives a deadline-bound context from parent (FEAT-008 FR-2).
func WithDeadline(parent context.Context, deadline time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, deadline)
}
