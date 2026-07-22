// Package logger provides structured JSON logging with per-request
// correlation IDs threaded through context, used across every other
// gateway component (FEAT-009).
package logger

import (
	"context"
	"log/slog"
	"os"
)

// base is the process-wide structured JSON logger. Every logger returned by
// FromContext derives from this handler so all log output shares the same
// format and destination.
var base = slog.New(slog.NewJSONHandler(os.Stdout, nil))

type correlationIDKey struct{}

// WithCorrelationID returns a context carrying the given correlation ID.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey{}, id)
}

// CorrelationIDFromContext retrieves a correlation ID attached by
// WithCorrelationID. ok is false if none is present.
func CorrelationIDFromContext(ctx context.Context) (id string, ok bool) {
	id, ok = ctx.Value(correlationIDKey{}).(string)
	return id, ok
}

// FromContext returns a structured logger bound to ctx's correlation ID, if
// any, so every log line it emits can be traced back to the request that
// produced it (FEAT-009 FR-1).
func FromContext(ctx context.Context) *slog.Logger {
	if id, ok := CorrelationIDFromContext(ctx); ok {
		return base.With("correlation_id", id)
	}
	return base
}

// Default returns the process-wide structured logger for call sites with no
// request context to draw a correlation ID from (e.g. startup, background
// jobs, one-off scripts), keeping their output structured JSON like every
// other log line (FEAT-009 Business Rules).
func Default() *slog.Logger {
	return base
}
