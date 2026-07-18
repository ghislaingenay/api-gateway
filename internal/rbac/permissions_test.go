package rbac

import (
	"testing"
)

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name       string
		role       *Role
		permission string
		want       bool
	}{
		{"nil role has no permissions", nil, "users:read", false},
		{"role without permission", &Role{Permissions: []string{"users:read"}}, "users:delete", false},
		{"role with permission", &Role{Permissions: []string{"users:read", "users:delete"}}, "users:delete", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPermission(tt.role, tt.permission); got != tt.want {
				t.Errorf("HasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}
