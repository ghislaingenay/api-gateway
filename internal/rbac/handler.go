package rbac

import (
	"encoding/json"
	"log"
	"net/http"
)

// RolesHandler returns an http.HandlerFunc for GET /roles, listing every
// role and its permissions from the injected RoleCache.
//
// Permission enforcement (roles:read) is not applied here — it is owned by
// the authorization middleware built in FEAT-003.
func RolesHandler(roles RoleCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		all := roles.All()
		response := make([]RoleResponse, 0, len(all))
		for _, role := range all {
			response = append(response, newRoleResponse(role))
		}
		writeJSON(w, http.StatusOK, response)
	}
}

// PermissionsHandler returns an http.HandlerFunc for GET /permissions,
// listing every permission from the injected RoleCache.
//
// Permission enforcement (roles:read) is not applied here — it is owned by
// the authorization middleware built in FEAT-003.
func PermissionsHandler(roles RoleCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		all := roles.AllPermissions()
		response := make([]PermissionResponse, 0, len(all))
		for _, permission := range all {
			response = append(response, newPermissionResponse(permission))
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("rbac: failed to write response: %v", err)
	}
}
