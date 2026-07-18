package rbac

// HasPermission reports whether the role includes the given permission.
func HasPermission(role *Role, permission string) bool {
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
