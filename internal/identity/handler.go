package identity

import (
	"encoding/json"
	"errors"
	"net/http"

	"api-gateway/internal/logger"
	"api-gateway/internal/profile"
	"api-gateway/internal/user"
)

// MeHandler returns an http.HandlerFunc for GET /auth/me. It requires a
// resolved identity (attached by ResolveMiddleware). Without a verified
// X-Tenant-ID, it lists every tenant the caller belongs to; with one, it
// returns the caller's tenant-scoped role/permissions/profile (FEAT-012
// FR-6).
func MeHandler(users user.Repository, profiles profile.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ident, ok := IndentityFromContext(r.Context())
		if !ok || ident == nil {
			writeUnauthorized(w, r)
			return
		}

		if ident.TenantID == nil {
			access, err := users.ListTenantAccess(r.Context(), ident.UserID)
			if err != nil {
				logger.FromContext(r.Context()).Error("identity: me: list tenant access", "user_id", ident.UserID.String(), "error", err.Error())
				writeError(w, r, http.StatusInternalServerError, "internal_error", "could not list tenant access")
				return
			}
			writeJSON(w, r, http.StatusOK, newMeWithoutTenantResponse(ident, access))
			return
		}

		p, err := profiles.GetByUserID(r.Context(), ident.UserID)
		if err != nil && !errors.Is(err, profile.ErrProfileNotFound) {
			logger.FromContext(r.Context()).Error("identity: me: load profile", "user_id", ident.UserID.String(), "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "internal_error", "could not load profile")
			return
		}
		if errors.Is(err, profile.ErrProfileNotFound) {
			p = nil
		}

		writeJSON(w, r, http.StatusOK, newCurrentUserResponse(ident, p))
	}
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.FromContext(r.Context()).Error("identity: failed to write response", "error", err.Error())
	}
}
