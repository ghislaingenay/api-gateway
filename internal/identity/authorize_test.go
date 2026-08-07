package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func validIdentity() *ResolvedIdentity {
	tenantID := uuid.New()
	roleID := uuid.New()
	return &ResolvedIdentity{
		UserID:      uuid.New(),
		Email:       "user@example.com",
		TenantID:    &tenantID,
		RoleID:      &roleID,
		RoleName:    "admin",
		Permissions: []string{"roles:read", "users:read"},
	}
}

func withIdentityCtx(ident *ResolvedIdentity) context.Context {
	return WithIdentity(context.Background(), ident)
}

func TestRequirePermission(t *testing.T) {
	tests := []struct {
		name           string
		ident          *ResolvedIdentity
		permission     string
		wantStatusCode int
		wantBody       map[string]string
	}{
		{
			name:           "no identity in context rejected",
			ident:          nil,
			permission:     "roles:read",
			wantStatusCode: http.StatusUnauthorized,
			wantBody:       map[string]string{"error": "unauthorized", "message": "invalid or missing token"},
		},
		{
			name: "identity missing the required permission rejected",
			ident: func() *ResolvedIdentity {
				i := validIdentity()
				i.Permissions = []string{"users:read"}
				return i
			}(),
			permission:     "roles:read",
			wantStatusCode: http.StatusForbidden,
			wantBody:       map[string]string{"error": "forbidden", "message": "insufficient permissions"},
		},
		{
			name: "identity with empty permissions rejected",
			ident: func() *ResolvedIdentity {
				i := validIdentity()
				i.Permissions = []string{}
				return i
			}(),
			permission:     "roles:read",
			wantStatusCode: http.StatusForbidden,
			wantBody:       map[string]string{"error": "forbidden", "message": "insufficient permissions"},
		},
		{
			name: "identity with the required permission accepted",
			ident: func() *ResolvedIdentity {
				i := validIdentity()
				i.Permissions = []string{"roles:read", "users:read"}
				return i
			}(),
			permission:     "roles:read",
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			middleware := RequirePermission(tt.permission)
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.ident != nil {
				req = req.WithContext(withIdentityCtx(tt.ident))
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatusCode, rec.Body.String())
			}
			if tt.wantBody != nil {
				assertJSONBody(t, rec.Body.Bytes(), tt.wantBody)
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name           string
		ident          *ResolvedIdentity
		roles          []string
		wantStatusCode int
		wantBody       map[string]string
	}{
		{
			name:           "no identity in context rejected",
			ident:          nil,
			roles:          []string{"admin"},
			wantStatusCode: http.StatusUnauthorized,
			wantBody:       map[string]string{"error": "unauthorized", "message": "invalid or missing token"},
		},
		{
			name: "identity with non-matching role rejected",
			ident: func() *ResolvedIdentity {
				i := validIdentity()
				i.RoleName = "manager"
				return i
			}(),
			roles:          []string{"admin"},
			wantStatusCode: http.StatusForbidden,
			wantBody:       map[string]string{"error": "forbidden", "message": "insufficient role"},
		},
		{
			name: "identity with empty role rejected",
			ident: func() *ResolvedIdentity {
				i := validIdentity()
				i.RoleName = ""
				return i
			}(),
			roles:          []string{"admin"},
			wantStatusCode: http.StatusForbidden,
			wantBody:       map[string]string{"error": "forbidden", "message": "insufficient role"},
		},
		{
			name: "identity with matching role accepted",
			ident: func() *ResolvedIdentity {
				i := validIdentity()
				i.RoleName = "admin"
				return i
			}(),
			roles:          []string{"admin", "manager"},
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			middleware := RequireRole(tt.roles...)
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.ident != nil {
				req = req.WithContext(withIdentityCtx(tt.ident))
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatusCode, rec.Body.String())
			}
			if tt.wantBody != nil {
				assertJSONBody(t, rec.Body.Bytes(), tt.wantBody)
			}
		})
	}
}

// TestRequirePermission_TypedNilIdentity covers a typed-nil
// *ResolvedIdentity stored in context (e.g. WithIdentity(ctx, nil)):
// FromContext's type assertion succeeds with ok=true, so the nil case must
// be checked explicitly to avoid a nil-pointer dereference on
// ident.Permissions.
func TestRequirePermission_TypedNilIdentity(t *testing.T) {
	t.Parallel()

	handler := RequirePermission("roles:read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(withIdentityCtx(nil))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (body: %s)", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	assertJSONBody(t, rec.Body.Bytes(), map[string]string{"error": "unauthorized", "message": "invalid or missing token"})
}

// TestRequireRole_TypedNilIdentity mirrors
// TestRequirePermission_TypedNilIdentity for RequireRole.
func TestRequireRole_TypedNilIdentity(t *testing.T) {
	t.Parallel()

	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(withIdentityCtx(nil))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (body: %s)", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	assertJSONBody(t, rec.Body.Bytes(), map[string]string{"error": "unauthorized", "message": "invalid or missing token"})
}

// TestRequireRoleThenRequirePermission covers the edge case of a route
// configured with both RequireRole and RequirePermission: each layer
// evaluates independently, so a role match followed by a missing
// permission still denies with 403.
func TestRequireRoleThenRequirePermission(t *testing.T) {
	t.Parallel()

	ident := validIdentity()
	ident.RoleName = "manager"
	ident.Permissions = []string{"users:read"}

	handler := RequireRole("admin", "manager")(
		RequirePermission("billing:read")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(withIdentityCtx(ident))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d (body: %s)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	assertJSONBody(t, rec.Body.Bytes(), map[string]string{"error": "forbidden", "message": "insufficient permissions"})
}

func assertJSONBody(t *testing.T, body []byte, want map[string]string) {
	t.Helper()
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v (body: %s)", err, body)
	}
	if len(got) != len(want) {
		t.Errorf("body = %v, want %v", got, want)
		return
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("body[%q] = %q, want %q", k, got[k], v)
		}
	}
}
