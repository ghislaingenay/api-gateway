package rbac

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

type fakeRoleCache struct {
	roles       []Role
	permissions []Permission
}

func (f *fakeRoleCache) GetRole(name string) (*Role, bool) {
	for i := range f.roles {
		if f.roles[i].Name == name {
			return &f.roles[i], true
		}
	}
	return nil, false
}

func (f *fakeRoleCache) All() []Role                  { return f.roles }
func (f *fakeRoleCache) AllPermissions() []Permission { return f.permissions }

func TestRolesHandler(t *testing.T) {
	tests := []struct {
		name      string
		cache     *fakeRoleCache
		wantCount int
	}{
		{
			name: "returns seeded roles",
			cache: &fakeRoleCache{
				roles: []Role{
					{ID: uuid.New(), Name: "admin", DisplayName: "Administrator", Description: "full access", Permissions: []string{"users:read"}, IsSystemRole: true},
					{ID: uuid.New(), Name: "viewer", DisplayName: "Viewer", Description: "read only", Permissions: []string{"users:read"}, IsSystemRole: true},
				},
			},
			wantCount: 2,
		},
		{
			name:      "empty cache returns empty list, not null",
			cache:     &fakeRoleCache{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/roles", nil)
			rec := httptest.NewRecorder()

			RolesHandler(tt.cache)(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			var got []RoleResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Errorf("len(response) = %d, want %d", len(got), tt.wantCount)
			}
			if got == nil {
				t.Errorf("response = nil, want non-nil empty slice")
			}
		})
	}
}

func TestPermissionsHandler(t *testing.T) {
	tests := []struct {
		name      string
		cache     *fakeRoleCache
		wantCount int
	}{
		{
			name: "returns seeded permissions",
			cache: &fakeRoleCache{
				permissions: []Permission{
					{ID: uuid.New(), Name: "users:read", Resource: "users", Action: "read", Description: "view users"},
					{ID: uuid.New(), Name: "roles:read", Resource: "roles", Action: "read", Description: "view roles"},
				},
			},
			wantCount: 2,
		},
		{
			name:      "empty cache returns empty list, not null",
			cache:     &fakeRoleCache{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/permissions", nil)
			rec := httptest.NewRecorder()

			PermissionsHandler(tt.cache)(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			var got []PermissionResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Errorf("len(response) = %d, want %d", len(got), tt.wantCount)
			}
			if got == nil {
				t.Errorf("response = nil, want non-nil empty slice")
			}
		})
	}
}
