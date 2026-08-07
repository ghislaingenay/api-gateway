package onboarding

import (
	"encoding/json"
	"net/http"

	"api-gateway/internal/identity"
	"api-gateway/internal/logger"
	"api-gateway/internal/rules"
)

// Handler returns an http.HandlerFunc for POST /onboarding: an
// authenticated caller with no tenant memberships creates a tenant and
// becomes its owner. Does not require X-Tenant-ID — there's no tenant yet.
func Handler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ident, ok := identity.IndentityFromContext(r.Context())
		if !ok || ident == nil {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "invalid or missing token")
			return
		}

		var req OnboardRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
			return
		}
		if err := rules.Validate(req); err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "organization_name is required")
			return
		}

		tenantID, err := service.Onboard(r.Context(), ident.UserID, req.OrganizationName)
		if err != nil {
			logger.FromContext(r.Context()).Error("onboarding: create tenant", "user_id", ident.UserID.String(), "error", err.Error())
			writeError(w, r, http.StatusInternalServerError, "internal_error", "could not create tenant")
			return
		}

		writeJSON(w, r, http.StatusCreated, OnboardResponse{
			TenantID: tenantID.String(),
			Role:     ownerRoleName,
		})
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, r, status, map[string]string{"error": code, "message": message})
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.FromContext(r.Context()).Error("onboarding: failed to write response", "error", err.Error())
	}
}
