package authhandler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/internal/auth"
	"api-gateway/internal/rbac"
	"api-gateway/internal/refreshtoken"
	"api-gateway/internal/tenant"
	"api-gateway/internal/user"

	"github.com/google/uuid"
)

// --- fakes ---

type fakeUserRepo struct {
	byID    map[uuid.UUID]*user.User
	byEmail map[string]*user.User // key: tenantID.String()+"|"+email
	updated map[uuid.UUID]time.Time
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byID:    map[uuid.UUID]*user.User{},
		byEmail: map[string]*user.User{},
		updated: map[uuid.UUID]time.Time{},
	}
}

func (f *fakeUserRepo) add(u user.User) {
	f.byID[u.ID] = &u
	f.byEmail[u.TenantID.String()+"|"+u.Email] = &u
}

func (f *fakeUserRepo) GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*user.User, error) {
	u, ok := f.byEmail[tenantID.String()+"|"+email]
	if !ok {
		return nil, user.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, user.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) UpdateLastLoginAt(ctx context.Context, id uuid.UUID, at time.Time) error {
	f.updated[id] = at
	return nil
}

type fakeTenantRepo struct {
	bySlug map[string]*tenant.Tenant
}

func (f *fakeTenantRepo) GetByID(ctx context.Context, id uuid.UUID) (*tenant.Tenant, error) {
	for _, t := range f.bySlug {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, tenant.ErrTenantNotFound
}

func (f *fakeTenantRepo) GetBySlug(ctx context.Context, slug string) (*tenant.Tenant, error) {
	t, ok := f.bySlug[slug]
	if !ok {
		return nil, tenant.ErrTenantNotFound
	}
	return t, nil
}

type fakeRefreshRepo struct {
	byHash map[string]*refreshtoken.RefreshToken
}

func newFakeRefreshRepo() *fakeRefreshRepo {
	return &fakeRefreshRepo{byHash: map[string]*refreshtoken.RefreshToken{}}
}

func (f *fakeRefreshRepo) Create(ctx context.Context, t refreshtoken.RefreshToken) error {
	f.byHash[t.TokenHash] = &t
	return nil
}

func (f *fakeRefreshRepo) GetByHash(ctx context.Context, hash string) (*refreshtoken.RefreshToken, error) {
	t, ok := f.byHash[hash]
	if !ok {
		return nil, refreshtoken.ErrNotFound
	}
	return t, nil
}

func (f *fakeRefreshRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	for _, t := range f.byHash {
		if t.ID == id {
			now := time.Now()
			t.RevokedAt = &now
		}
	}
	return nil
}

type fakeRoleCache struct {
	roles []rbac.Role
}

func (f *fakeRoleCache) GetRole(name string) (*rbac.Role, bool) {
	for i := range f.roles {
		if f.roles[i].Name == name {
			return &f.roles[i], true
		}
	}
	return nil, false
}

func (f *fakeRoleCache) GetRoleByID(id uuid.UUID) (*rbac.Role, bool) {
	for i := range f.roles {
		if f.roles[i].ID == id {
			return &f.roles[i], true
		}
	}
	return nil, false
}

func (f *fakeRoleCache) All() []rbac.Role                  { return f.roles }
func (f *fakeRoleCache) AllPermissions() []rbac.Permission { return nil }

type fakeSigner struct{ signed int }

func (f *fakeSigner) Sign(claims auth.CustomClaims) (string, error) {
	f.signed++
	return fmt.Sprintf("signed-token-%d-%s", f.signed, claims.UserID), nil
}

// --- fixtures ---

func newFixtures() (tenantID, roleID, userID uuid.UUID, tenants *fakeTenantRepo, users *fakeUserRepo, roles *fakeRoleCache) {
	tenantID = uuid.New()
	roleID = uuid.New()
	userID = uuid.New()

	tenants = &fakeTenantRepo{bySlug: map[string]*tenant.Tenant{
		"acme": {ID: tenantID, Slug: "acme", Name: "Acme"},
	}}

	hash, err := auth.HashPassword("correct-password")
	if err != nil {
		panic(err)
	}

	users = newFakeUserRepo()
	users.add(user.User{
		ID:           userID,
		TenantID:     tenantID,
		RoleID:       roleID,
		Email:        "user@acme.test",
		PasswordHash: hash,
		IsActive:     true,
	})

	roles = &fakeRoleCache{roles: []rbac.Role{
		{ID: roleID, Name: "viewer", Permissions: []string{"users:read"}},
	}}

	return tenantID, roleID, userID, tenants, users, roles
}

// --- tests ---

func TestLoginHandler(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		mutate         func(users *fakeUserRepo, userID uuid.UUID)
		wantStatusCode int
		wantErrorCode  string
	}{
		{
			name:           "success",
			body:           `{"email":"user@acme.test","password":"correct-password","tenant_slug":"acme"}`,
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "wrong password",
			body:           `{"email":"user@acme.test","password":"wrong","tenant_slug":"acme"}`,
			wantStatusCode: http.StatusUnauthorized,
			wantErrorCode:  "invalid_credentials",
		},
		{
			name:           "unknown email",
			body:           `{"email":"nobody@acme.test","password":"correct-password","tenant_slug":"acme"}`,
			wantStatusCode: http.StatusUnauthorized,
			wantErrorCode:  "invalid_credentials",
		},
		{
			name:           "unknown tenant slug",
			body:           `{"email":"user@acme.test","password":"correct-password","tenant_slug":"nope"}`,
			wantStatusCode: http.StatusUnauthorized,
			wantErrorCode:  "invalid_credentials",
		},
		{
			name: "inactive user",
			body: `{"email":"user@acme.test","password":"correct-password","tenant_slug":"acme"}`,
			mutate: func(users *fakeUserRepo, userID uuid.UUID) {
				users.byID[userID].IsActive = false
			},
			wantStatusCode: http.StatusUnauthorized,
			wantErrorCode:  "invalid_credentials",
		},
		{
			name:           "malformed json",
			body:           `not-json`,
			wantStatusCode: http.StatusBadRequest,
			wantErrorCode:  "invalid_request",
		},
		{
			name:           "missing email fails validation",
			body:           `{"password":"correct-password","tenant_slug":"acme"}`,
			wantStatusCode: http.StatusBadRequest,
			wantErrorCode:  "invalid_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, userID, tenants, users, roles := newFixtures()
			if tt.mutate != nil {
				tt.mutate(users, userID)
			}
			refreshTokens := newFakeRefreshRepo()
			signer := &fakeSigner{}

			req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()

			LoginHandler(users, tenants, refreshTokens, roles, signer)(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tt.wantStatusCode, rec.Body.String())
			}

			if tt.wantErrorCode != "" {
				var got map[string]string
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if got["error"] != tt.wantErrorCode {
					t.Errorf("error = %q, want %q", got["error"], tt.wantErrorCode)
				}
				return
			}

			var got LoginResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.AccessToken == "" || got.RefreshToken == "" {
				t.Errorf("LoginResponse = %+v, want non-empty tokens", got)
			}
			if got.TokenType != "Bearer" {
				t.Errorf("TokenType = %q, want Bearer", got.TokenType)
			}
			if len(refreshTokens.byHash) != 1 {
				t.Errorf("stored refresh tokens = %d, want 1", len(refreshTokens.byHash))
			}
			if len(users.updated) != 1 {
				t.Errorf("UpdateLastLoginAt calls = %d, want 1", len(users.updated))
			}
		})
	}
}

func TestRefreshHandler(t *testing.T) {
	t.Run("success rotates the token", func(t *testing.T) {
		_, _, userID, _, users, roles := newFixtures()
		refreshTokens := newFakeRefreshRepo()
		signer := &fakeSigner{}

		raw, hash, err := auth.GenerateRefreshToken()
		if err != nil {
			t.Fatalf("GenerateRefreshToken() error = %v", err)
		}
		oldID := uuid.New()
		refreshTokens.byHash[hash] = &refreshtoken.RefreshToken{
			ID: oldID, UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
		}

		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(
			fmt.Sprintf(`{"refresh_token":%q}`, raw)))
		rec := httptest.NewRecorder()

		RefreshHandler(refreshTokens, users, roles, signer)(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}

		var got RefreshResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.AccessToken == "" || got.RefreshToken == "" {
			t.Errorf("RefreshResponse = %+v, want non-empty tokens", got)
		}
		if got.RefreshToken == raw {
			t.Error("new refresh token equals old raw token, want rotation")
		}

		if refreshTokens.byHash[hash].RevokedAt == nil {
			t.Error("old refresh token not revoked after rotation")
		}

		// Reusing the old (now revoked) token must fail.
		req2 := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(
			fmt.Sprintf(`{"refresh_token":%q}`, raw)))
		rec2 := httptest.NewRecorder()
		RefreshHandler(refreshTokens, users, roles, signer)(rec2, req2)
		if rec2.Code != http.StatusUnauthorized {
			t.Errorf("reused token status = %d, want 401", rec2.Code)
		}
	})

	t.Run("unknown token rejected", func(t *testing.T) {
		_, _, _, _, users, roles := newFixtures()
		refreshTokens := newFakeRefreshRepo()
		signer := &fakeSigner{}

		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(`{"refresh_token":"does-not-exist"}`))
		rec := httptest.NewRecorder()
		RefreshHandler(refreshTokens, users, roles, signer)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("expired token rejected", func(t *testing.T) {
		_, _, userID, _, users, roles := newFixtures()
		refreshTokens := newFakeRefreshRepo()
		signer := &fakeSigner{}

		raw, hash, _ := auth.GenerateRefreshToken()
		refreshTokens.byHash[hash] = &refreshtoken.RefreshToken{
			ID: uuid.New(), UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(-time.Hour),
		}

		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(
			fmt.Sprintf(`{"refresh_token":%q}`, raw)))
		rec := httptest.NewRecorder()
		RefreshHandler(refreshTokens, users, roles, signer)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("revoked token rejected", func(t *testing.T) {
		_, _, userID, _, users, roles := newFixtures()
		refreshTokens := newFakeRefreshRepo()
		signer := &fakeSigner{}

		raw, hash, _ := auth.GenerateRefreshToken()
		revokedAt := time.Now()
		refreshTokens.byHash[hash] = &refreshtoken.RefreshToken{
			ID: uuid.New(), UserID: userID, TokenHash: hash,
			ExpiresAt: time.Now().Add(time.Hour), RevokedAt: &revokedAt,
		}

		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(
			fmt.Sprintf(`{"refresh_token":%q}`, raw)))
		rec := httptest.NewRecorder()
		RefreshHandler(refreshTokens, users, roles, signer)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestLogoutHandler(t *testing.T) {
	t.Run("revokes the given token", func(t *testing.T) {
		_, _, userID, _, _, _ := newFixtures()
		refreshTokens := newFakeRefreshRepo()

		raw, hash, _ := auth.GenerateRefreshToken()
		refreshTokens.byHash[hash] = &refreshtoken.RefreshToken{
			ID: uuid.New(), UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
		}

		req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewBufferString(
			fmt.Sprintf(`{"refresh_token":%q}`, raw)))
		rec := httptest.NewRecorder()
		LogoutHandler(refreshTokens)(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		if refreshTokens.byHash[hash].RevokedAt == nil {
			t.Error("token not revoked after logout")
		}
	})

	t.Run("unknown token is idempotent, still 200", func(t *testing.T) {
		refreshTokens := newFakeRefreshRepo()

		req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewBufferString(`{"refresh_token":"does-not-exist"}`))
		rec := httptest.NewRecorder()
		LogoutHandler(refreshTokens)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})
}

func TestMeHandler(t *testing.T) {
	t.Run("success returns user without password_hash", func(t *testing.T) {
		tenantID, roleID, userID, _, users, roles := newFixtures()

		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		req = req.WithContext(auth.WithClaims(req.Context(), &auth.CustomClaims{
			TenantID: tenantID, UserID: userID, RoleID: roleID, Role: "viewer",
		}))
		rec := httptest.NewRecorder()

		MeHandler(users, roles)(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		if bytes.Contains(rec.Body.Bytes(), []byte("password")) {
			t.Errorf("response contains password field: %s", rec.Body.String())
		}

		var got UserResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.Email != "user@acme.test" {
			t.Errorf("Email = %q, want user@acme.test", got.Email)
		}
	})

	t.Run("missing claims rejected", func(t *testing.T) {
		_, _, _, _, users, roles := newFixtures()

		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		rec := httptest.NewRecorder()
		MeHandler(users, roles)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("deactivated user rejected", func(t *testing.T) {
		tenantID, roleID, userID, _, users, roles := newFixtures()
		users.byID[userID].IsActive = false

		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		req = req.WithContext(auth.WithClaims(req.Context(), &auth.CustomClaims{
			TenantID: tenantID, UserID: userID, RoleID: roleID, Role: "viewer",
		}))
		rec := httptest.NewRecorder()
		MeHandler(users, roles)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}
