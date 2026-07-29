package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/config"
	"api-gateway/internal/rbac"
	"api-gateway/internal/refreshtoken"
	"api-gateway/internal/tenant"
	"api-gateway/internal/user"

	"github.com/google/uuid"
)

// testCookieConfig is a minimal CookieConfig used across handler tests.
var testCookieConfig = &config.CookieConfig{Secure: true, SameSite: http.SameSiteLaxMode}

// refreshCookieFrom extracts the refresh token cookie from a recorded
// response, if any.
func refreshCookieFrom(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == config.RefreshTokenCookieName {
			return c
		}
	}
	return nil
}

// requestWithRefreshCookie builds a request carrying the given raw refresh
// token as a cookie, the way a browser would after LoginHandler/
// RefreshHandler set it.
func requestWithRefreshCookie(method, target, rawToken string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.AddCookie(&http.Cookie{Name: config.RefreshTokenCookieName, Value: rawToken})
	return req
}

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

func (f *fakeRoleCache) Refresh(ctx context.Context) error { return nil }

type fakeSigner struct{ signed int }

func (f *fakeSigner) Sign(claims CustomClaims) (string, error) {
	f.signed++
	return fmt.Sprintf("signed-token-%d-%s", f.signed, claims.UserID), nil
}

// fakeGuard records RegisterFailure/Reset calls per key so tests can assert
// on login-failure tracking without a real Redis instance.
type fakeGuard struct {
	failures map[string]int
	resets   int
}

func newFakeGuard() *fakeGuard {
	return &fakeGuard{failures: map[string]int{}}
}

func (f *fakeGuard) RegisterFailure(ctx context.Context, key string) (int, error) {
	f.failures[key]++
	return f.failures[key], nil
}

func (f *fakeGuard) Reset(ctx context.Context, key string) error {
	f.resets++
	delete(f.failures, key)
	return nil
}

// --- fixtures ---

func newFixtures() (tenantID, roleID, userID uuid.UUID, tenants *fakeTenantRepo, users *fakeUserRepo, roles *fakeRoleCache) {
	tenantID = uuid.New()
	roleID = uuid.New()
	userID = uuid.New()

	tenants = &fakeTenantRepo{bySlug: map[string]*tenant.Tenant{
		"acme": {ID: tenantID, Slug: "acme", Name: "Acme"},
	}}

	hash, err := HashPassword("correct-password")
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
			guard := newFakeGuard()

			req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()

			LoginHandler(users, tenants, refreshTokens, roles, signer, guard, config.LoginSecurityConfig{}, 0, testCookieConfig)(rec, req)

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
			if got.AccessToken == "" {
				t.Errorf("LoginResponse = %+v, want non-empty access token", got)
			}
			if got.TokenType != "Bearer" {
				t.Errorf("TokenType = %q, want Bearer", got.TokenType)
			}
			cookie := refreshCookieFrom(rec)
			if cookie == nil || cookie.Value == "" {
				t.Fatal("no refresh_token cookie set")
			}
			if !cookie.HttpOnly {
				t.Error("refresh_token cookie not HttpOnly")
			}
			if !cookie.Secure {
				t.Error("refresh_token cookie not Secure")
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

func TestLoginHandler_ProgressiveDelay(t *testing.T) {
	_, _, _, tenants, users, roles := newFixtures()
	refreshTokens := newFakeRefreshRepo()
	signer := &fakeSigner{}
	guard := newFakeGuard()
	delayCfg := config.LoginSecurityConfig{
		DelayBaseBackoff: 20 * time.Millisecond,
		DelayMax:         200 * time.Millisecond,
	}

	body := `{"email":"user@acme.test","password":"wrong","tenant_slug":"acme"}`
	key := "192.0.2.1:acme:user@acme.test"

	// First failure: RegisterFailure returns attempt 1, a non-zero delay is
	// possible (jittered, may be 0..BaseBackoff) but the guard must record it.
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	req.RemoteAddr = "192.0.2.1:12345"
	rec := httptest.NewRecorder()
	LoginHandler(users, tenants, refreshTokens, roles, signer, guard, delayCfg, 0, testCookieConfig)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if guard.failures[key] != 1 {
		t.Fatalf("failures[%q] = %d, want 1", key, guard.failures[key])
	}

	// A handful more failures should keep incrementing the same key's count.
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
		req.RemoteAddr = "192.0.2.1:12345"
		rec := httptest.NewRecorder()
		LoginHandler(users, tenants, refreshTokens, roles, signer, guard, delayCfg, 0, testCookieConfig)(rec, req)
	}
	if guard.failures[key] != 4 {
		t.Fatalf("failures[%q] = %d, want 4", key, guard.failures[key])
	}

	// A subsequent successful login must reset the failure count for that key.
	successBody := `{"email":"user@acme.test","password":"correct-password","tenant_slug":"acme"}`
	req = httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(successBody))
	req.RemoteAddr = "192.0.2.1:12345"
	rec = httptest.NewRecorder()
	LoginHandler(users, tenants, refreshTokens, roles, signer, guard, delayCfg, 0, testCookieConfig)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if guard.resets != 1 {
		t.Fatalf("resets = %d, want 1", guard.resets)
	}
	if _, ok := guard.failures[key]; ok {
		t.Errorf("failures[%q] still present after reset", key)
	}
}

func TestClampAttempt(t *testing.T) {
	t.Parallel()

	base := 250 * time.Millisecond
	max := 4 * time.Second

	tests := []struct {
		name    string
		attempt int
		want    int
	}{
		{"first attempt", 1, 1},
		{"below zero clamps to 1", 0, 1},
		{"grows normally while under max", 3, 3},
		// base<<4 == 4s == max, so 5 is the last attempt that still changes
		// the shifted value; anything beyond must clamp to it.
		{"at the doubling that reaches max", 5, 5},
		{"just past the cap clamps to 5", 6, 5},
		// Without clamping, resilience.Backoff's BaseBackoff<<(attempt-1)
		// overflows int64 well before this and can wrap negative/zero,
		// silently disabling the delay — this is the regression guard.
		{"far past the cap still clamps to 5", 1000, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := clampAttempt(base, max, tt.attempt); got != tt.want {
				t.Errorf("clampAttempt(%v, %v, %d) = %d, want %d", base, max, tt.attempt, got, tt.want)
			}
		})
	}
}

func TestDelayOnFailure_NoOverflowForSustainedAttacker(t *testing.T) {
	t.Parallel()

	guard := newFakeGuard()
	// Tiny durations so 200 real (bounded) sleeps stay fast; what matters is
	// the ratio (37+ doublings before max) that used to overflow, not the
	// absolute scale.
	delayCfg := config.LoginSecurityConfig{
		DelayBaseBackoff: 1 * time.Millisecond,
		DelayMax:         5 * time.Millisecond,
	}

	// Drive the same key well past the attempt count where the unclamped
	// shift would overflow (see TestClampAttempt for the exact numbers) and
	// confirm delayOnFailure keeps returning within a bounded time — the
	// actual "does the delay stay non-zero/bounded" guarantee is verified
	// deterministically in TestClampAttempt; this just guards against a
	// hang or runaway wait if that clamp were ever removed.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			delayOnFailure(context.Background(), guard, delayCfg, "attacker-key")
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("delayOnFailure did not return promptly across 200 sustained failures; possible overflow regression")
	}
}

func TestRefreshHandler(t *testing.T) {
	t.Run("success rotates the token", func(t *testing.T) {
		_, _, userID, _, users, roles := newFixtures()
		refreshTokens := newFakeRefreshRepo()
		signer := &fakeSigner{}

		raw, hash, err := GenerateRefreshToken()
		if err != nil {
			t.Fatalf("GenerateRefreshToken() error = %v", err)
		}
		oldID := uuid.New()
		refreshTokens.byHash[hash] = &refreshtoken.RefreshToken{
			ID: oldID, UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
		}

		req := requestWithRefreshCookie(http.MethodPost, "/auth/refresh", raw)
		rec := httptest.NewRecorder()

		RefreshHandler(refreshTokens, users, roles, signer, testCookieConfig)(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}

		var got RefreshResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.AccessToken == "" {
			t.Errorf("RefreshResponse = %+v, want non-empty access token", got)
		}
		newCookie := refreshCookieFrom(rec)
		if newCookie == nil || newCookie.Value == "" {
			t.Fatal("no refresh_token cookie set on refresh")
		}
		if newCookie.Value == raw {
			t.Error("new refresh token equals old raw token, want rotation")
		}

		if refreshTokens.byHash[hash].RevokedAt == nil {
			t.Error("old refresh token not revoked after rotation")
		}

		// Reusing the old (now revoked) token must fail.
		req2 := requestWithRefreshCookie(http.MethodPost, "/auth/refresh", raw)
		rec2 := httptest.NewRecorder()
		RefreshHandler(refreshTokens, users, roles, signer, testCookieConfig)(rec2, req2)
		if rec2.Code != http.StatusUnauthorized {
			t.Errorf("reused token status = %d, want 401", rec2.Code)
		}
	})

	t.Run("unknown token rejected", func(t *testing.T) {
		_, _, _, _, users, roles := newFixtures()
		refreshTokens := newFakeRefreshRepo()
		signer := &fakeSigner{}

		req := requestWithRefreshCookie(http.MethodPost, "/auth/refresh", "does-not-exist")
		rec := httptest.NewRecorder()
		RefreshHandler(refreshTokens, users, roles, signer, testCookieConfig)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("missing cookie rejected", func(t *testing.T) {
		_, _, _, _, users, roles := newFixtures()
		refreshTokens := newFakeRefreshRepo()
		signer := &fakeSigner{}

		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
		rec := httptest.NewRecorder()
		RefreshHandler(refreshTokens, users, roles, signer, testCookieConfig)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("expired token rejected", func(t *testing.T) {
		_, _, userID, _, users, roles := newFixtures()
		refreshTokens := newFakeRefreshRepo()
		signer := &fakeSigner{}

		raw, hash, _ := GenerateRefreshToken()
		refreshTokens.byHash[hash] = &refreshtoken.RefreshToken{
			ID: uuid.New(), UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(-time.Hour),
		}

		req := requestWithRefreshCookie(http.MethodPost, "/auth/refresh", raw)
		rec := httptest.NewRecorder()
		RefreshHandler(refreshTokens, users, roles, signer, testCookieConfig)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("revoked token rejected", func(t *testing.T) {
		_, _, userID, _, users, roles := newFixtures()
		refreshTokens := newFakeRefreshRepo()
		signer := &fakeSigner{}

		raw, hash, _ := GenerateRefreshToken()
		revokedAt := time.Now()
		refreshTokens.byHash[hash] = &refreshtoken.RefreshToken{
			ID: uuid.New(), UserID: userID, TokenHash: hash,
			ExpiresAt: time.Now().Add(time.Hour), RevokedAt: &revokedAt,
		}

		req := requestWithRefreshCookie(http.MethodPost, "/auth/refresh", raw)
		rec := httptest.NewRecorder()
		RefreshHandler(refreshTokens, users, roles, signer, testCookieConfig)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}

func TestLogoutHandler(t *testing.T) {
	t.Run("revokes the given token and clears the cookie", func(t *testing.T) {
		_, _, userID, _, _, _ := newFixtures()
		refreshTokens := newFakeRefreshRepo()

		raw, hash, _ := GenerateRefreshToken()
		refreshTokens.byHash[hash] = &refreshtoken.RefreshToken{
			ID: uuid.New(), UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
		}

		req := requestWithRefreshCookie(http.MethodPost, "/auth/logout", raw)
		rec := httptest.NewRecorder()
		LogoutHandler(refreshTokens, testCookieConfig)(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		if refreshTokens.byHash[hash].RevokedAt == nil {
			t.Error("token not revoked after logout")
		}
		cleared := refreshCookieFrom(rec)
		if cleared == nil || cleared.MaxAge >= 0 {
			t.Errorf("refresh_token cookie not cleared, got %+v", cleared)
		}
	})

	t.Run("unknown token is idempotent, still 200", func(t *testing.T) {
		refreshTokens := newFakeRefreshRepo()

		req := requestWithRefreshCookie(http.MethodPost, "/auth/logout", "does-not-exist")
		rec := httptest.NewRecorder()
		LogoutHandler(refreshTokens, testCookieConfig)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("missing cookie is idempotent, still 200", func(t *testing.T) {
		refreshTokens := newFakeRefreshRepo()

		req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
		rec := httptest.NewRecorder()
		LogoutHandler(refreshTokens, testCookieConfig)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})
}

func TestMeHandler(t *testing.T) {
	t.Run("success returns user without password_hash", func(t *testing.T) {
		tenantID, roleID, userID, _, users, roles := newFixtures()

		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		req = req.WithContext(WithClaims(req.Context(), &CustomClaims{
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
		req = req.WithContext(WithClaims(req.Context(), &CustomClaims{
			TenantID: tenantID, UserID: userID, RoleID: roleID, Role: "viewer",
		}))
		rec := httptest.NewRecorder()
		MeHandler(users, roles)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}
