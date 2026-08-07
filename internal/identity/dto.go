package identity

import (
	"api-gateway/internal/profile"
	"api-gateway/internal/user"
)

// TenantAccessResponse is one tenant a caller belongs to, returned by
// GET /auth/me when no X-Tenant-ID was supplied.
type TenantAccessResponse struct {
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

// MeWithoutTenantResponse is the body of GET /auth/me when the caller
// didn't supply X-Tenant-ID: identity plus every tenant they belong to.
type MeWithoutTenantResponse struct {
	UserID  string                 `json:"user_id"`
	Email   string                 `json:"email"`
	Tenants []TenantAccessResponse `json:"tenants"`
}

func newMeWithoutTenantResponse(ident *ResolvedIdentity, access []user.TenantAccess) MeWithoutTenantResponse {
	tenants := make([]TenantAccessResponse, 0, len(access))
	for _, a := range access {
		tenants = append(tenants, TenantAccessResponse{
			TenantID: a.TenantID.String(),
			Name:     a.TenantName,
			Role:     a.RoleName,
		})
	}
	return MeWithoutTenantResponse{
		UserID:  ident.UserID.String(),
		Email:   ident.Email,
		Tenants: tenants,
	}
}

// ProfileResponse is the profile subset returned as part of a
// tenant-scoped GET /auth/me response.
type ProfileResponse struct {
	DisplayName string `json:"display_name"`
	Timezone    string `json:"timezone"`
}

// CurrentUserResponse is the body of GET /auth/me when the caller
// supplied a verified X-Tenant-ID: identity, tenant-scoped role/
// permissions, and profile.
type CurrentUserResponse struct {
	UserID      string          `json:"user_id"`
	Email       string          `json:"email"`
	TenantID    string          `json:"tenant_id"`
	Role        string          `json:"role"`
	Permissions []string        `json:"permissions"`
	Profile     ProfileResponse `json:"profile"`
}

func newCurrentUserResponse(ident *ResolvedIdentity, p *profile.Profile) CurrentUserResponse {
	resp := CurrentUserResponse{
		UserID:      ident.UserID.String(),
		Email:       ident.Email,
		TenantID:    ident.TenantID.String(),
		Role:        ident.RoleName,
		Permissions: ident.Permissions,
	}
	if p != nil {
		resp.Profile = ProfileResponse{
			DisplayName: displayName(p),
			Timezone:    p.Timezone,
		}
	}
	return resp
}

// displayName combines a profile's first/last name into a single display
// string, tolerating either half being unset.
func displayName(p *profile.Profile) string {
	var first, last string
	if p.FirstName != nil {
		first = *p.FirstName
	}
	if p.LastName != nil {
		last = *p.LastName
	}
	switch {
	case first != "" && last != "":
		return first + " " + last
	case first != "":
		return first
	default:
		return last
	}
}
