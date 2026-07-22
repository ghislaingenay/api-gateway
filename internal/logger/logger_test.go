package logger

import (
	"context"
	"testing"
)

func TestWithCorrelationID_CorrelationIDFromContext(t *testing.T) {
	t.Parallel()

	t.Run("round trips a stored id", func(t *testing.T) {
		t.Parallel()
		ctx := WithCorrelationID(context.Background(), "abc-123")
		id, ok := CorrelationIDFromContext(ctx)
		if !ok || id != "abc-123" {
			t.Fatalf("CorrelationIDFromContext() = (%q, %v), want (\"abc-123\", true)", id, ok)
		}
	})

	t.Run("absent on a bare context", func(t *testing.T) {
		t.Parallel()
		_, ok := CorrelationIDFromContext(context.Background())
		if ok {
			t.Fatal("expected ok = false for a context with no correlation ID")
		}
	})
}

func TestFromContext(t *testing.T) {
	t.Parallel()

	t.Run("returns a logger even without a correlation ID", func(t *testing.T) {
		t.Parallel()
		if got := FromContext(context.Background()); got == nil {
			t.Fatal("FromContext() = nil")
		}
	})

	t.Run("returns a logger when a correlation ID is present", func(t *testing.T) {
		t.Parallel()
		ctx := WithCorrelationID(context.Background(), "abc-123")
		if got := FromContext(ctx); got == nil {
			t.Fatal("FromContext() = nil")
		}
	})
}
