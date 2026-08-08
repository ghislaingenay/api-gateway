package onboarding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"api-gateway/internal/identity"

	"github.com/google/uuid"
)

type fakeService struct {
	tenantID uuid.UUID
	err      error
	gotOrg   string
}

func (f *fakeService) Onboard(ctx context.Context, userID uuid.UUID, organizationName string) (uuid.UUID, error) {
	f.gotOrg = organizationName
	return f.tenantID, f.err
}

func TestHandler_Unauthorized(t *testing.T) {
	t.Parallel()

	handler := Handler(&fakeService{})
	req := httptest.NewRequest(http.MethodPost, "/onboarding", bytes.NewBufferString(`{"organization_name":"Acme"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandler_MalformedBody(t *testing.T) {
	t.Parallel()

	handler := Handler(&fakeService{})
	req := withIdentity(httptest.NewRequest(http.MethodPost, "/onboarding", bytes.NewBufferString(`not json`)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandler_MissingOrganizationName(t *testing.T) {
	t.Parallel()

	handler := Handler(&fakeService{})
	req := withIdentity(httptest.NewRequest(http.MethodPost, "/onboarding", bytes.NewBufferString(`{}`)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandler_TenantLimitReached(t *testing.T) {
	t.Parallel()

	handler := Handler(&fakeService{err: ErrTenantLimitReached})
	req := withIdentity(httptest.NewRequest(http.MethodPost, "/onboarding", bytes.NewBufferString(`{"organization_name":"Acme"}`)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandler_ServiceError(t *testing.T) {
	t.Parallel()

	handler := Handler(&fakeService{err: errors.New("db down")})
	req := withIdentity(httptest.NewRequest(http.MethodPost, "/onboarding", bytes.NewBufferString(`{"organization_name":"Acme"}`)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandler_Success(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	svc := &fakeService{tenantID: tenantID}
	handler := Handler(svc)
	req := withIdentity(httptest.NewRequest(http.MethodPost, "/onboarding", bytes.NewBufferString(`{"organization_name":"Acme"}`)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if svc.gotOrg != "Acme" {
		t.Errorf("Onboard() called with organization_name = %q, want %q", svc.gotOrg, "Acme")
	}

	var resp OnboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.TenantID != tenantID.String() {
		t.Errorf("TenantID = %q, want %q", resp.TenantID, tenantID.String())
	}
	if resp.Role != "owner" {
		t.Errorf("Role = %q, want owner", resp.Role)
	}
}

func withIdentity(r *http.Request) *http.Request {
	ident := &identity.ResolvedIdentity{UserID: uuid.New(), Email: "user@example.com"}
	return r.WithContext(identity.WithIdentity(r.Context(), ident))
}
