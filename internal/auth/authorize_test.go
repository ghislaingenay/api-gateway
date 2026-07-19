package auth

import (
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
	}{
		{
			name:           "no claims in context rejected",
			claims:         nil,
			permission:     "roles:read",
			wantStatusCode: http.StatusUnauthorized,
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
		})
	}
}
