package user_test

import (
	"encoding/json"
	"strings"
	"testing"

	"api-gateway/internal/testfixtures"
	"api-gateway/internal/user"
)

func validUser() user.User {
	return testfixtures.NewValidUser()
}

func TestUser_KeycloakSubNeverSerialized(t *testing.T) {
	u := validUser()
	u.KeycloakSub = "some-keycloak-subject"

	data, err := json.Marshal(&u)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if strings.Contains(string(data), "some-keycloak-subject") {
		t.Errorf("expected keycloak_sub not to appear in JSON output, got: %s", data)
	}
	if strings.Contains(string(data), "keycloak_sub") {
		t.Errorf("expected keycloak_sub key not to appear in JSON output, got: %s", data)
	}
}
