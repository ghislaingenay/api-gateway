package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequirePermission(t *testing.T) {
	tests := []struct {
		name           string
		claims         *CustomClaims
		permission     string
		wantStatusCode int
		wantBody       map[string]string
	}{
		{
			name:           "no claims in context rejected",
			claims:         nil,
			permission:     "roles:read",
			wantStatusCode: http.StatusUnauthorized,
			wantBody:       map[string]string{"error": "unauthorized", "message": "invalid or missing token"},
		},
		{
			name: "claims missing the required permission rejected",
			claims: func() *CustomClaims {
				c := validClaims()
				c.Permissions = []string{"users:read"}
				return &c
			}(),
			permission:     "roles:read",
			wantStatusCode: http.StatusForbidden,
			wantBody:       map[string]string{"error": "forbidden", "message": "insufficient permissions"},
		},
		{
			name: "claims with empty permissions rejected",
			claims: func() *CustomClaims {
				c := validClaims()
				c.Permissions = []string{}
				return &c
			}(),
			permission:     "roles:read",
			wantStatusCode: http.StatusForbidden,
			wantBody:       map[string]string{"error": "forbidden", "message": "insufficient permissions"},
		},
		{
			name: "claims with the required permission accepted",
			claims: func() *CustomClaims {
				c := validClaims()
				c.Permissions = []string{"roles:read", "users:read"}
				return &c
			}(),
			permission:     "roles:read",
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			middleware := RequirePermission(tt.permission)
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.claims != nil {
				req = req.WithContext(WithClaims(req.Context(), tt.claims))
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
		claims         *CustomClaims
		roles          []string
		wantStatusCode int
		wantBody       map[string]string
	}{
		{
			name:           "no claims in context rejected",
			claims:         nil,
			roles:          []string{"admin"},
			wantStatusCode: http.StatusUnauthorized,
			wantBody:       map[string]string{"error": "unauthorized", "message": "invalid or missing token"},
		},
		{
			name: "claims with non-matching role rejected",
			claims: func() *CustomClaims {
				c := validClaims()
				c.Role = "manager"
				return &c
			}(),
			roles:          []string{"admin"},
			wantStatusCode: http.StatusForbidden,
			wantBody:       map[string]string{"error": "forbidden", "message": "insufficient role"},
		},
		{
			name: "claims with empty role rejected",
			claims: func() *CustomClaims {
				c := validClaims()
				c.Role = ""
				return &c
			}(),
			roles:          []string{"admin"},
			wantStatusCode: http.StatusForbidden,
			wantBody:       map[string]string{"error": "forbidden", "message": "insufficient role"},
		},
		{
			name: "claims with matching role accepted",
			claims: func() *CustomClaims {
				c := validClaims()
				c.Role = "admin"
				return &c
			}(),
			roles:          []string{"admin", "manager"},
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			middleware := RequireRole(tt.roles...)
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.claims != nil {
				req = req.WithContext(WithClaims(req.Context(), tt.claims))
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

// TestRequireRoleThenRequirePermission covers the edge case of a route
// configured with both RequireRole and RequirePermission: each layer
// evaluates independently, so a role match followed by a missing
// permission still denies with 403.
func TestRequireRoleThenRequirePermission(t *testing.T) {
	t.Parallel()

	claims := validClaims()
	claims.Role = "manager"
	claims.Permissions = []string{"users:read"}

	handler := RequireRole("admin", "manager")(
		RequirePermission("billing:read")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithClaims(req.Context(), &claims))

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
