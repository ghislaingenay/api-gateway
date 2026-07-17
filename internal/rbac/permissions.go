// Package rbac provides permission-checking helpers over the core identity
// models. It contains no schema or persistence concerns of its own.
package rbac

import "api-gateway/internal/models"

// HasPermission reports whether the role includes the given permission.
func HasPermission(role *models.Role, permission string) bool {
	if role == nil {
		return false
	}
	for _, p := range role.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// UserHasPermission reports whether the user's role includes the given permission.
func UserHasPermission(user *models.User, permission string) bool {
	if user == nil {
		return false
	}
	return HasPermission(user.Role, permission)
}

// IsAdmin reports whether the user has the admin role.
func IsAdmin(user *models.User) bool {
	return user != nil && user.Role != nil && user.Role.Name == "admin"
}

// IsManager reports whether the user has the manager role.
func IsManager(user *models.User) bool {
	return user != nil && user.Role != nil && user.Role.Name == "manager"
}

// IsViewer reports whether the user has the viewer role.
func IsViewer(user *models.User) bool {
	return user != nil && user.Role != nil && user.Role.Name == "viewer"
}

// CanManageUsers reports whether the user can manage other users.
func CanManageUsers(user *models.User) bool {
	return IsAdmin(user) || IsManager(user)
}
