package audit

import (
	"context"
	"testing"

	"api-gateway/internal/logger"

	"github.com/google/uuid"
)

func TestLogAuthzDecision(t *testing.T) {
	t.Parallel()

	// LogAuthzDecision only writes to the structured logger; this test
	// exercises both branches to guard against a panic on nil/zero values,
	// with and without a correlation ID in context.
	LogAuthzDecision(context.Background(), true, uuid.New(), uuid.New(), "roles:read")
	LogAuthzDecision(context.Background(), false, uuid.Nil, uuid.Nil, "roles:read")

	ctx := logger.WithCorrelationID(context.Background(), "abc-123")
	LogAuthzDecision(ctx, false, uuid.New(), uuid.New(), "roles:read")
}
