package user

import (
	"api-gateway/internal/rbac"
)

// HasPermission reports whether the user's role includes the given permission.
func (u *User) HasPermission(permission string) bool {
	if u == nil {
		return false
	}
	return rbac.HasPermission(u.Role, permission)
}

// IsAdmin reports whether the user has the admin role.
func (u *User) IsAdmin() bool {
	return u != nil && u.Role != nil && u.Role.Name == "admin"
}

// IsManager reports whether the user has the manager role.
func (u *User) IsManager() bool {
	return u != nil && u.Role != nil && u.Role.Name == "manager"
}

// IsViewer reports whether the user has the viewer role.
func (u *User) IsViewer() bool {
	return u != nil && u.Role != nil && u.Role.Name == "viewer"
}

// CanManageUsers reports whether the user can manage other users.
func (u *User) CanManageUsers() bool {
	return u.IsAdmin() || u.IsManager()
}
