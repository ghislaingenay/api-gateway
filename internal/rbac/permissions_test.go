package rbac

import (
	"testing"

	"api-gateway/internal/models"
)

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name       string
		role       *models.Role
		permission string
		want       bool
	}{
		{"nil role has no permissions", nil, "users:read", false},
		{"role without permission", &models.Role{Permissions: []string{"users:read"}}, "users:delete", false},
		{"role with permission", &models.Role{Permissions: []string{"users:read", "users:delete"}}, "users:delete", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPermission(tt.role, tt.permission); got != tt.want {
				t.Errorf("HasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserHasPermission(t *testing.T) {
	tests := []struct {
		name string
		user *models.User
		want bool
	}{
		{"nil user", nil, false},
		{"user without role", &models.User{}, false},
		{"user with role and permission", &models.User{Role: &models.Role{Permissions: []string{"users:read"}}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UserHasPermission(tt.user, "users:read"); got != tt.want {
				t.Errorf("UserHasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRoleChecks(t *testing.T) {
	admin := &models.User{Role: &models.Role{Name: "admin"}}
	manager := &models.User{Role: &models.Role{Name: "manager"}}
	viewer := &models.User{Role: &models.Role{Name: "viewer"}}
	noRole := &models.User{}

	tests := []struct {
		name string
		fn   func(*models.User) bool
		user *models.User
		want bool
	}{
		{"IsAdmin true for admin", IsAdmin, admin, true},
		{"IsAdmin false for manager", IsAdmin, manager, false},
		{"IsAdmin false for nil user", IsAdmin, nil, false},
		{"IsManager true for manager", IsManager, manager, true},
		{"IsManager false for viewer", IsManager, viewer, false},
		{"IsViewer true for viewer", IsViewer, viewer, true},
		{"IsViewer false for no role", IsViewer, noRole, false},
		{"CanManageUsers true for admin", CanManageUsers, admin, true},
		{"CanManageUsers true for manager", CanManageUsers, manager, true},
		{"CanManageUsers false for viewer", CanManageUsers, viewer, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(tt.user); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
