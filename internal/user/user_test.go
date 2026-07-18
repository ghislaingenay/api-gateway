package user_test

import (
	"encoding/json"
	"strings"
	"testing"

	"api-gateway/internal/rbac"
	"api-gateway/internal/testfixtures"
	"api-gateway/internal/user"
)

func validUser() user.User {
	return testfixtures.NewValidUser()
}

func TestUser_PasswordHashNeverSerialized(t *testing.T) {
	u := validUser()
	u.PasswordHash = "super-secret-hash"

	data, err := json.Marshal(&u)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if strings.Contains(string(data), "super-secret-hash") {
		t.Errorf("expected password hash not to appear in JSON output, got: %s", data)
	}
	if strings.Contains(string(data), "password_hash") {
		t.Errorf("expected password_hash key not to appear in JSON output, got: %s", data)
	}
}

func TestUser_RelationshipsOmittedWhenNil(t *testing.T) {
	u := validUser()

	data, err := json.Marshal(&u)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	for _, field := range []string{"tenant", "role", "profile"} {
		if strings.Contains(string(data), `"`+field+`"`) {
			t.Errorf("expected nil relationship %q to be omitted from JSON output, got: %s", field, data)
		}
	}
}

func TestUser_RelationshipsPopulatedWhenSet(t *testing.T) {
	u := validUser()
	tenant := testfixtures.NewValidTenant()
	role := rbac.Role{Name: "admin", DisplayName: "Administrator", Description: "Full access"}
	profile := testfixtures.NewValidProfile()

	u.Tenant = &tenant
	u.Role = &role
	u.Profile = &profile

	data, err := json.Marshal(&u)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	for _, field := range []string{"tenant", "role", "profile"} {
		if !strings.Contains(string(data), `"`+field+`"`) {
			t.Errorf("expected populated relationship %q to appear in JSON output, got: %s", field, data)
		}
	}
}
