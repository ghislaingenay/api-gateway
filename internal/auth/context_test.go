package auth

import (
	"context"
	"testing"
)

func TestClaimsFromContext(t *testing.T) {
	claims := &CustomClaims{Email: "user@example.com"}

	if _, ok := ClaimsFromContext(context.Background()); ok {
		t.Error("ClaimsFromContext() ok = true on empty context, want false")
	}

	ctx := WithClaims(context.Background(), claims)
	got, ok := ClaimsFromContext(ctx)
	if !ok {
		t.Fatal("ClaimsFromContext() ok = false, want true")
	}
	if got != claims {
		t.Errorf("ClaimsFromContext() = %v, want %v", got, claims)
	}
}
