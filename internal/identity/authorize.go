package identity

import (
	"net/http"
	"slices"
	"strings"

	"api-gateway/internal/audit"

	"github.com/google/uuid"
)

// RequirePermission returns middleware that rejects requests whose resolved
// identity (attached by ResolveMiddleware) doesn't include the given
// permission. It must run after ResolveMiddleware in the chain, since it
// reads identity from the request context rather than resolving it itself.
func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ident, ok := FromContext(r.Context())
			if !ok || ident == nil {
				writeUnauthorized(w, r)
				return
			}
			if !slices.Contains(ident.Permissions, permission) {
				audit.LogAuthzDecision(r.Context(), false, tenantIDOrNil(ident), ident.UserID, permission)
				writeForbidden(w, r, "insufficient permissions")
				return
			}
			audit.LogAuthzDecision(r.Context(), true, tenantIDOrNil(ident), ident.UserID, permission)
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole returns middleware that rejects requests whose resolved
// identity (attached by ResolveMiddleware) doesn't carry one of the
// allowed roles. It must run after ResolveMiddleware in the chain, since it
// reads identity from the request context rather than resolving it itself.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	required := strings.Join(roles, ",")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ident, ok := FromContext(r.Context())
			if !ok || ident == nil {
				writeUnauthorized(w, r)
				return
			}
			if !slices.Contains(roles, ident.RoleName) {
				audit.LogAuthzDecision(r.Context(), false, tenantIDOrNil(ident), ident.UserID, required)
				writeForbidden(w, r, "insufficient role")
				return
			}
			audit.LogAuthzDecision(r.Context(), true, tenantIDOrNil(ident), ident.UserID, required)
			next.ServeHTTP(w, r)
		})
	}
}

// tenantIDOrNil returns ident.TenantID's value for audit logging, or
// uuid.Nil when no tenant was resolved on this request (e.g. before
// onboarding).
func tenantIDOrNil(ident *ResolvedIdentity) uuid.UUID {
	if ident.TenantID == nil {
		return uuid.Nil
	}
	return *ident.TenantID
}
