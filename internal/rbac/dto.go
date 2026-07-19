package rbac

// RoleResponse is the API representation of a Role returned by GET /roles.
type RoleResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	DisplayName  string   `json:"display_name"`
	Description  string   `json:"description"`
	Permissions  []string `json:"permissions"`
	IsSystemRole bool     `json:"is_system_role"`
}

// PermissionResponse is the API representation of a Permission returned by
// GET /permissions.
type PermissionResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

func newRoleResponse(r Role) RoleResponse {
	return RoleResponse{
		ID:           r.ID.String(),
		Name:         r.Name,
		DisplayName:  r.DisplayName,
		Description:  r.Description,
		Permissions:  r.Permissions,
		IsSystemRole: r.IsSystemRole,
	}
}

func newPermissionResponse(p Permission) PermissionResponse {
	return PermissionResponse{
		ID:          p.ID.String(),
		Name:        p.Name,
		Resource:    p.Resource,
		Action:      p.Action,
		Description: p.Description,
	}
}
