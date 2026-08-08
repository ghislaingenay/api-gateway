package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"api-gateway/internal/audit"
	"api-gateway/internal/identity"
	"api-gateway/internal/logger"

	"github.com/google/uuid"
)

// TenantHeader is the trusted internal header the gateway sets on every
// proxied request, carrying the tenant_id resolved from validated claims.
// It is the only source downstream services may trust for tenant identity.
const TenantHeader = "X-Gateway-Tenant-ID"

// clientTenantHeaders are headers a client might supply to spoof the
// trusted downstream tenant header. They are always stripped before a
// request is forwarded downstream, regardless of their value. Unlike
// pre-FEAT-012 (ADR-003), X-Tenant-ID is not stripped here — it's now
// legitimate, server-verified client input (re-verified against
// tenant_users by identity.ResolveMiddleware on every request), not a
// spoofing vector; only the trusted internal TenantHeader needs
// protecting from a client trying to set it directly.
var clientTenantHeaders = []string{TenantHeader}

// TenantStatusChecker reports whether a tenant is active. Declared here
// (the consumer) rather than in the tenant package per the DI convention;
// *tenant.redisStatusCache satisfies it structurally.
type TenantStatusChecker interface {
	IsActive(ctx context.Context, tenantID uuid.UUID) (bool, error)
}

// NewHandler returns the gateway's core request handler: it resolves the
// tenant from the resolved identity, rejects inactive tenants, resolves the
// downstream route from the static RouteTable, strips any client-supplied
// tenant headers, sets the trusted TenantHeader, and proxies the request
// upstream. It must run after identity.ResolveMiddleware and
// identity.RequireTenant, since it reads a required, already-verified
// TenantID from the request context rather than parsing the token or
// re-verifying tenant membership itself.
func NewHandler(routes *RouteTable, statusChecker TenantStatusChecker, proxy Proxier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ident, ok := identity.IdentityFromContext(r.Context())
		if !ok || ident == nil || ident.TenantID == nil {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing authenticated identity")
			return
		}
		tenantID := *ident.TenantID

		active, err := statusChecker.IsActive(r.Context(), tenantID)
		if err != nil {
			logger.FromContext(r.Context()).Error("gateway: tenant status check failed", "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "internal_error", "failed to verify tenant status")
			return
		}
		if !active {
			writeError(w, r, http.StatusForbidden, "tenant_inactive", "tenant is not active")
			return
		}

		route, ok := RouteFromContext(r.Context())
		if !ok {
			route, ok = routes.Resolve(r.Method, r.URL.Path)
			if !ok {
				writeError(w, r, http.StatusNotFound, "not_found", "no matching route")
				return
			}
			// Share the resolved route via context so a decorating Proxier
			// (e.g. the FEAT-008 resilient proxy) can read its per-route
			// Timeout/RetryMaxAttempts without re-resolving it.
			r = r.WithContext(WithRoute(r.Context(), route))
		}

		if missing := missingPermissions(ident.Permissions, route.PermissionsRequired); len(missing) > 0 {
			audit.LogAuthzDecision(r.Context(), false, tenantID, ident.UserID, strings.Join(route.PermissionsRequired, ","))
			writeForbidden(w, r, "insufficient permissions")
			return
		}
		if len(route.PermissionsRequired) > 0 {
			audit.LogAuthzDecision(r.Context(), true, tenantID, ident.UserID, strings.Join(route.PermissionsRequired, ","))
		}

		for _, header := range clientTenantHeaders {
			r.Header.Del(header)
		}
		r.Header.Set(TenantHeader, tenantID.String())

		proxy.Proxy(w, r, route.Upstream)
	})
}

// missingPermissions returns the subset of required permissions the caller's
// granted permissions don't cover. A route with no PermissionsRequired is
// satisfied by any caller (nil result).
func missingPermissions(granted, required []string) []string {
	var missing []string
	for _, permission := range required {
		if !slices.Contains(granted, permission) {
			missing = append(missing, permission)
		}
	}
	return missing
}

func writeForbidden(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error":   "forbidden",
		"message": message,
	}); err != nil {
		logger.FromContext(r.Context()).Error("gateway: failed to write forbidden response", "error", err.Error())
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	}); err != nil {
		logger.FromContext(r.Context()).Error("gateway: failed to write error response", "error", err.Error())
	}
}
