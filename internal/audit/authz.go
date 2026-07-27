// Package audit records authorization decisions for compliance review.
package audit

import (
	"context"

	"api-gateway/internal/logger"

	"github.com/google/uuid"
)

// Structured log wrapper method records an allow/deny authorization decision against a
// required permission or role
func LogAuthzDecision(ctx context.Context, allowed bool, tenantID, userID uuid.UUID, required string) {
	result := "deny"
	if allowed {
		result = "allow"
	}
	logger.FromContext(ctx).Info("authz decision",
		"event_type", "authz_"+result,
		"result", result,
		"tenant_id", tenantID.String(),
		"user_id", userID.String(),
		"required", required,
	)
}
