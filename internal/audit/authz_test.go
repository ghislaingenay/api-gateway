package audit

import (
	"testing"

	"github.com/google/uuid"
)

func TestLogAuthzDecision(t *testing.T) {
	t.Parallel()

	// LogAuthzDecision only writes to the standard logger; this test
	// exercises both branches to guard against a panic on nil/zero values.
	LogAuthzDecision(true, uuid.New(), uuid.New(), "roles:read")
	LogAuthzDecision(false, uuid.Nil, uuid.Nil, "roles:read")
}
