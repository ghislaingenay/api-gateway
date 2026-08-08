package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/internal/profile"
	"api-gateway/internal/user"

	"github.com/google/uuid"
)

type fakeUserRepo struct {
	access    []user.TenantAccess
	accessErr error
}

func (f *fakeUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	return nil, user.ErrUserNotFound
}
func (f *fakeUserRepo) GetByKeycloakSub(ctx context.Context, sub string) (*user.User, error) {
	return nil, user.ErrUserNotFound
}
func (f *fakeUserRepo) EnsureByKeycloakSub(ctx context.Context, sub, email string) (*user.User, error) {
	return nil, user.ErrUserNotFound
}
func (f *fakeUserRepo) ListTenantAccess(ctx context.Context, userID uuid.UUID) ([]user.TenantAccess, error) {
	return f.access, f.accessErr
}

type fakeProfileRepo struct {
	profile *profile.Profile
	err     error
}

func (f *fakeProfileRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (*profile.Profile, error) {
	return f.profile, f.err
}

func TestMeHandler_Unauthorized(t *testing.T) {
	t.Parallel()

	handler := MeHandler(&fakeUserRepo{}, &fakeProfileRepo{})

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMeHandler_WithoutTenant(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	tenantID := uuid.New()
	repo := &fakeUserRepo{access: []user.TenantAccess{
		{TenantID: tenantID, TenantName: "Acme Inc", RoleName: "owner"},
	}}
	handler := MeHandler(repo, &fakeProfileRepo{})

	ident := &ResolvedIdentity{UserID: userID, Email: "user@example.com"}
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req = req.WithContext(WithIdentity(req.Context(), ident))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got MeWithoutTenantResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.UserID != userID.String() {
		t.Errorf("UserID = %q, want %q", got.UserID, userID.String())
	}
	if len(got.Tenants) != 1 || got.Tenants[0].Role != "owner" {
		t.Errorf("Tenants = %+v, want one entry with role owner", got.Tenants)
	}
}

func TestMeHandler_WithTenant(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	tenantID := uuid.New()
	roleID := uuid.New()
	firstName := "Ada"
	p := &profile.Profile{UserID: userID, FirstName: &firstName, Timezone: "UTC"}
	handler := MeHandler(&fakeUserRepo{}, &fakeProfileRepo{profile: p})

	ident := &ResolvedIdentity{
		UserID:      userID,
		Email:       "user@example.com",
		TenantID:    &tenantID,
		RoleID:      &roleID,
		RoleName:    "owner",
		Permissions: []string{"tenants:read"},
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req = req.WithContext(WithIdentity(req.Context(), ident))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got CurrentUserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.TenantID != tenantID.String() {
		t.Errorf("TenantID = %q, want %q", got.TenantID, tenantID.String())
	}
	if got.Role != "owner" {
		t.Errorf("Role = %q, want owner", got.Role)
	}
	if got.Profile.DisplayName != "Ada" {
		t.Errorf("Profile.DisplayName = %q, want Ada", got.Profile.DisplayName)
	}
}

func TestMeHandler_WithTenant_NoProfile(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	tenantID := uuid.New()
	handler := MeHandler(&fakeUserRepo{}, &fakeProfileRepo{err: profile.ErrProfileNotFound})

	ident := &ResolvedIdentity{UserID: userID, TenantID: &tenantID, RoleName: "owner"}
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req = req.WithContext(WithIdentity(req.Context(), ident))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
}
