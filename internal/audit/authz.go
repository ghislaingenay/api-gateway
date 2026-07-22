// Package audit records authorization decisions for compliance review.
package audit

import (
	"context"

	"api-gateway/internal/logger"

	"github.com/google/uuid"
)

// LogAuthzDecision records an allow/deny authorization decision against a
// required permission or role, identifying the tenant and user involved.
// event_type is "authz_allow" or "authz_deny" so denials are greppable in
// aggregated logs (FEAT-009 FR-4). ctx supplies the request's correlation
// ID via logger.FromContext.
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
