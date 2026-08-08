package identity

import "github.com/google/uuid"

// ResolvedIdentity is the caller's identity as verified for this request.
// UserID/Email come from the validated JWT (via JIT provisioning).
// TenantID/RoleID/RoleName/Permissions stay nil/empty until a verified
// X-Tenant-ID header has been resolved against tenant_users — e.g. before
// onboarding, or on requests that don't need tenant context.
type ResolvedIdentity struct {
	UserID      uuid.UUID
	Email       string
	TenantID    *uuid.UUID
	RoleID      *uuid.UUID
	RoleName    string
	Permissions []string
}
