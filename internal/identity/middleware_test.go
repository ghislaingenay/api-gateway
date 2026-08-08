package identity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/internal/auth"
	"api-gateway/internal/rbac"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type fakeResolver struct {
	userID        uuid.UUID
	ensureErr     error
	membership    *TenantUser
	membershipErr error
}

func (f *fakeResolver) EnsureUser(ctx context.Context, keycloakSub, email string) (uuid.UUID, error) {
	if f.ensureErr != nil {
		return uuid.Nil, f.ensureErr
	}
	return f.userID, nil
}

func (f *fakeResolver) ResolveTenantUser(ctx context.Context, userID, tenantID uuid.UUID) (*TenantUser, error) {
	if f.membershipErr != nil {
		return nil, f.membershipErr
	}
	return f.membership, nil
}

type fakeRoleCache struct {
	roles map[uuid.UUID]*rbac.Role
}

func (f *fakeRoleCache) GetRole(name string) (*rbac.Role, bool) { return nil, false }
func (f *fakeRoleCache) GetRoleByID(id uuid.UUID) (*rbac.Role, bool) {
	r, ok := f.roles[id]
	return r, ok
}
func (f *fakeRoleCache) All() []rbac.Role                  { return nil }
func (f *fakeRoleCache) AllPermissions() []rbac.Permission { return nil }
func (f *fakeRoleCache) Refresh(ctx context.Context) error { return nil }

func requestWithClaims(sub, email string) *http.Request {
	claims := &auth.CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: sub},
		Email:            email,
	}
	req := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
	return req.WithContext(auth.WithClaims(req.Context(), claims))
}

func TestResolveMiddleware_NoClaims(t *testing.T) {
	t.Parallel()

	resolver := &fakeResolver{}
	cache := NewTenantUserCache(resolver, nil, 0)
	roles := &fakeRoleCache{}
	handler := ResolveMiddleware(resolver, cache, roles)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestResolveMiddleware_NoTenantHeader_AttachesIdentityWithoutTenant(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	resolver := &fakeResolver{userID: userID}
	cache := NewTenantUserCache(resolver, nil, 0)
	roles := &fakeRoleCache{}

	var got *ResolvedIdentity
	handler := ResolveMiddleware(resolver, cache, roles)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = IdentityFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, requestWithClaims("sub-1", "user@example.com"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got == nil {
		t.Fatal("expected identity attached to context")
	}
	if got.UserID != userID {
		t.Errorf("UserID = %v, want %v", got.UserID, userID)
	}
	if got.TenantID != nil {
		t.Errorf("TenantID = %v, want nil", got.TenantID)
	}
}

func TestResolveMiddleware_EnsureUserError(t *testing.T) {
	t.Parallel()

	resolver := &fakeResolver{ensureErr: errors.New("db down")}
	cache := NewTenantUserCache(resolver, nil, 0)
	roles := &fakeRoleCache{}
	handler := ResolveMiddleware(resolver, cache, roles)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, requestWithClaims("sub-1", "user@example.com"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestResolveMiddleware_InvalidTenantHeader(t *testing.T) {
	t.Parallel()

	resolver := &fakeResolver{userID: uuid.New()}
	cache := NewTenantUserCache(resolver, nil, 0)
	roles := &fakeRoleCache{}
	handler := ResolveMiddleware(resolver, cache, roles)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := requestWithClaims("sub-1", "user@example.com")
	req.Header.Set(tenantHeaderName, "not-a-uuid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestResolveMiddleware_NotMember(t *testing.T) {
	t.Parallel()

	resolver := &fakeResolver{userID: uuid.New(), membershipErr: ErrNotMember}
	cache := NewTenantUserCache(resolver, nil, 0)
	roles := &fakeRoleCache{}
	handler := ResolveMiddleware(resolver, cache, roles)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := requestWithClaims("sub-1", "user@example.com")
	req.Header.Set(tenantHeaderName, uuid.New().String())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestResolveMiddleware_VerifiedTenant_AttachesRoleAndPermissions(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	tenantID := uuid.New()
	roleID := uuid.New()
	resolver := &fakeResolver{userID: userID, membership: &TenantUser{UserID: userID, RoleID: roleID}}
	cache := NewTenantUserCache(resolver, nil, 0)
	roles := &fakeRoleCache{roles: map[uuid.UUID]*rbac.Role{
		roleID: {ID: roleID, Name: "owner", Permissions: []string{"tenants:read"}},
	}}

	var got *ResolvedIdentity
	handler := ResolveMiddleware(resolver, cache, roles)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = IdentityFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := requestWithClaims("sub-1", "user@example.com")
	req.Header.Set(tenantHeaderName, tenantID.String())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got == nil || got.TenantID == nil || *got.TenantID != tenantID {
		t.Fatalf("expected TenantID %v attached, got %+v", tenantID, got)
	}
	if got.RoleName != "owner" {
		t.Errorf("RoleName = %q, want owner", got.RoleName)
	}
	if len(got.Permissions) != 1 || got.Permissions[0] != "tenants:read" {
		t.Errorf("Permissions = %v, want [tenants:read]", got.Permissions)
	}
}

func TestRequireTenant_NoIdentity(t *testing.T) {
	t.Parallel()

	handler := RequireTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireTenant_NoTenantID_Returns400(t *testing.T) {
	t.Parallel()

	handler := RequireTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	ident := &ResolvedIdentity{UserID: uuid.New()}
	req := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
	req = req.WithContext(WithIdentity(req.Context(), ident))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRequireTenant_WithTenantID_Passes(t *testing.T) {
	t.Parallel()

	called := false
	handler := RequireTenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	tenantID := uuid.New()
	ident := &ResolvedIdentity{UserID: uuid.New(), TenantID: &tenantID}
	req := httptest.NewRequest(http.MethodGet, "/api/thing", nil)
	req = req.WithContext(WithIdentity(req.Context(), ident))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
