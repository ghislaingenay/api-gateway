package validation

import (
	"testing"

	"api-gateway/internal/profile"
	"api-gateway/internal/rbac"
	"api-gateway/internal/tenant"
	"api-gateway/internal/testfixtures"
	"api-gateway/internal/user"

	"github.com/google/uuid"
)

func validTenant() tenant.Tenant {
	return tenant.Tenant{
		ID:                 uuid.New(),
		Name:               "Acme Inc",
		Slug:               "acme-inc",
		Tier:               "free",
		RateLimitPerMinute: 60,
		RateLimitPerHour:   1000,
		MaxUsers:           10,
	}
}

func TestValidate_Tenant(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*tenant.Tenant)
		wantErr bool
	}{
		{"valid tenant", func(tenant *tenant.Tenant) {}, false},
		{"empty name fails", func(tenant *tenant.Tenant) { tenant.Name = "" }, true},
		{"out of range tier fails", func(tenant *tenant.Tenant) { tenant.Tier = "ultra" }, true},
		{"invalid slug with uppercase fails", func(tenant *tenant.Tenant) { tenant.Slug = "Acme-Inc" }, true},
		{"invalid slug with leading hyphen fails", func(tenant *tenant.Tenant) { tenant.Slug = "-acme" }, true},
		{"zero rate limit fails", func(tenant *tenant.Tenant) { tenant.RateLimitPerMinute = 0 }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant := testfixtures.NewValidTenant()
			tt.mutate(&tenant)

			err := Validate(&tenant)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_Profile(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*profile.Profile)
		wantErr bool
	}{
		{"valid profile", func(p *profile.Profile) {}, false},
		{"invalid timezone fails", func(p *profile.Profile) { p.Timezone = "Mars/OlympusMons" }, true},
		{"missing timezone fails", func(p *profile.Profile) { p.Timezone = "" }, true},
		{"missing user id fails", func(p *profile.Profile) { p.UserID = uuid.Nil }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := testfixtures.NewValidProfile()
			tt.mutate(&profile)

			err := Validate(&profile)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_Role(t *testing.T) {
	tests := []struct {
		name    string
		role    rbac.Role
		wantErr bool
	}{
		{"valid role", rbac.Role{Name: "admin", DisplayName: "Administrator", Description: "Full access"}, false},
		{"invalid role name fails", rbac.Role{Name: "superuser", DisplayName: "Super User", Description: "desc"}, true},
		{"empty description fails", rbac.Role{Name: "admin", DisplayName: "Administrator", Description: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func validUser() user.User {
	return user.User{
		ID:           uuid.New(),
		TenantID:     uuid.New(),
		RoleID:       uuid.New(),
		Email:        "user@example.com",
		PasswordHash: "hash",
	}
}

func TestValidate_User(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*user.User)
		wantErr bool
	}{
		{"valid user", func(u *user.User) {}, false},
		{"malformed email fails", func(u *user.User) { u.Email = "not-an-email" }, true},
		{"empty email fails", func(u *user.User) { u.Email = "" }, true},
		{"missing tenant id fails", func(u *user.User) { u.TenantID = uuid.Nil }, true},
		{"missing role id fails", func(u *user.User) { u.RoleID = uuid.Nil }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := validUser()
			tt.mutate(&user)

			err := Validate(&user)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}