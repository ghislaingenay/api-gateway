// Package audit records authorization decisions for compliance review.
package audit

import (
	"log"

	"github.com/google/uuid"
)

// LogAuthzDecision records an allow/deny authorization decision against a
// required permission or role, identifying the tenant and user involved.
//
// Correlation IDs are intentionally not included here: FEAT-009
// (Observability) owns correlation ID generation/propagation and has not
// been implemented yet. Once it lands, callers can thread a correlation ID
// through without changing this package's dependents.
func LogAuthzDecision(allowed bool, tenantID, userID uuid.UUID, required string) {
	result := "deny"
	if allowed {
		result = "allow"
	}
	log.Printf("authz decision result=%s tenant_id=%s user_id=%s required=%s", result, tenantID, userID, required)
}
