package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"strings"

	"api-gateway/internal/audit"
)

// RequirePermission returns middleware that rejects requests whose validated
// claims (attached by JWTAuthMiddleware) don't include the given permission.
// It must run after JWTAuthMiddleware in the chain, since it reads claims
// from the request context rather than parsing the token itself.
func RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				writeUnauthorized(w)
				return
			}
			if !slices.Contains(claims.Permissions, permission) {
				audit.LogAuthzDecision(false, claims.TenantID, claims.UserID, permission)
				writeForbidden(w, "insufficient permissions")
				return
			}
			audit.LogAuthzDecision(true, claims.TenantID, claims.UserID, permission)
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole returns middleware that rejects requests whose validated
// claims (attached by JWTAuthMiddleware) don't carry one of the allowed
// roles. It must run after JWTAuthMiddleware in the chain, since it reads
// claims from the request context rather than parsing the token itself.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	required := strings.Join(roles, ",")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				writeUnauthorized(w)
				return
			}
			if !slices.Contains(roles, claims.Role) {
				audit.LogAuthzDecision(false, claims.TenantID, claims.UserID, required)
				writeForbidden(w, "insufficient role")
				return
			}
			audit.LogAuthzDecision(true, claims.TenantID, claims.UserID, required)
			next.ServeHTTP(w, r)
		})
	}
}

func writeForbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error":   "forbidden",
		"message": message,
	}); err != nil {
		log.Printf("failed to write forbidden response: %v", err)
	}
}
