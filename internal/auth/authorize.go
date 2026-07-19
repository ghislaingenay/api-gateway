package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"slices"
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
				writeForbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error":   "forbidden",
		"message": "missing required permission",
	}); err != nil {
		log.Printf("failed to write forbidden response: %v", err)
	}
}
