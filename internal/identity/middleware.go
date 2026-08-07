package identity

import (
	"encoding/json"
	"errors"
	"net/http"

	"api-gateway/internal/auth"
	"api-gateway/internal/logger"
	"api-gateway/internal/rbac"

	"github.com/google/uuid"
)

// tenantHeaderName is the client-supplied header naming which tenant the
// caller wants to act in. It is never trusted directly — ResolveMiddleware
// re-verifies it against tenant_users on every request before any role or
// permission is resolved (TD-012 §7).
const tenantHeaderName = "X-Tenant-ID"

// ResolveMiddleware JIT-provisions a local users row for the request's
// validated Keycloak sub, then — if X-Tenant-ID is present — verifies
// tenant membership and resolves the caller's role/permissions, attaching
// the result as a ResolvedIdentity to the request context. It must run
// after auth.JWTAuthMiddleware, since it reads claims from the request
// context rather than parsing the token itself.
func ResolveMiddleware(resolver Resolver, tenantUsers *TenantUserCache, roles rbac.RoleCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok || claims == nil {
				writeUnauthorized(w, r)
				return
			}

			userID, err := resolver.EnsureUser(r.Context(), claims.Subject, claims.Email)
			if err != nil {
				logger.FromContext(r.Context()).Error("identity: ensure user", "error", err.Error())
				writeError(w, r, http.StatusInternalServerError, "internal_error", "could not resolve identity")
				return
			}

			tenantIDRaw := r.Header.Get(tenantHeaderName)
			if tenantIDRaw == "" {
				next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), &ResolvedIdentity{
					UserID: userID,
					Email:  claims.Email,
				})))
				return
			}

			tenantID, err := uuid.Parse(tenantIDRaw)
			if err != nil {
				writeError(w, r, http.StatusBadRequest, "invalid_request", "X-Tenant-ID must be a valid UUID")
				return
			}

			tu, err := tenantUsers.Resolve(r.Context(), claims.Subject, userID, tenantID)
			if err != nil {
				if errors.Is(err, ErrNotMember) {
					writeForbidden(w, r, "not a member of this tenant")
					return
				}
				logger.FromContext(r.Context()).Error("identity: resolve tenant user", "error", err.Error())
				writeError(w, r, http.StatusInternalServerError, "internal_error", "could not resolve tenant membership")
				return
			}

			role, ok := roles.GetRoleByID(tu.RoleID)
			var roleName string
			var permissions []string
			if ok {
				roleName = role.Name
				permissions = role.Permissions
			} else {
				logger.FromContext(r.Context()).Error("identity: tenant_users references unknown role_id", "role_id", tu.RoleID.String())
			}

			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), &ResolvedIdentity{
				UserID:      userID,
				Email:       claims.Email,
				TenantID:    &tenantID,
				RoleID:      &tu.RoleID,
				RoleName:    roleName,
				Permissions: permissions,
			})))
		})
	}
}

// RequireTenant returns middleware that rejects requests with 400 when no
// verified X-Tenant-ID was resolved onto the request's identity. It must
// run after ResolveMiddleware, and is only wired into chains that need
// tenant context (e.g. /api/*) — GET /auth/me and POST /onboarding
// deliberately don't require one (FEAT-012 FR-2/FR-4).
func RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ident, ok := IdentityFromContext(r.Context())
		if !ok || ident == nil {
			writeUnauthorized(w, r)
			return
		}
		if ident.TenantID == nil {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "X-Tenant-ID header is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusUnauthorized, "unauthorized", "invalid or missing token")
}

func writeForbidden(w http.ResponseWriter, r *http.Request, message string) {
	writeError(w, r, http.StatusForbidden, "forbidden", message)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	}); err != nil {
		logger.FromContext(r.Context()).Error("identity: failed to write error response", "error", err.Error())
	}
}
