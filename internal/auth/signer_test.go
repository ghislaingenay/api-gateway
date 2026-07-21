package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewSigner_SignAndVerify(t *testing.T) {
	t.Parallel()

	key := generateRSAKeyPair(t)
	privB64 := encodePrivateKeyPEM(t, key)

	signer, err := NewSigner("test-kid", privB64)
	if err != nil {
		t.Fatalf("NewSigner() error = %v", err)
	}

	claims := validClaims()
	tokenString, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	keyStore := newTestKeyStore(t, "test-kid", &key.PublicKey)

	parsed := &CustomClaims{}
	_, err = jwt.ParseWithClaims(tokenString, parsed, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		return keyStore.GetKey(kid)
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		t.Fatalf("ParseWithClaims() error = %v", err)
	}

	if parsed.UserID != claims.UserID {
		t.Errorf("parsed.UserID = %v, want %v", parsed.UserID, claims.UserID)
	}
	if parsed.TenantID != claims.TenantID {
		t.Errorf("parsed.TenantID = %v, want %v", parsed.TenantID, claims.TenantID)
	}
}

func TestNewSigner_InvalidPrivateKey(t *testing.T) {
	t.Parallel()

	if _, err := NewSigner("kid", "not-base64-pem!!!"); err == nil {
		t.Error("NewSigner() with invalid key error = nil, want error")
	}
}

func TestNewSigner_MissingKID(t *testing.T) {
	t.Parallel()

	key := generateRSAKeyPair(t)
	privB64 := encodePrivateKeyPEM(t, key)

	if _, err := NewSigner("", privB64); err == nil {
		t.Error("NewSigner() with empty kid error = nil, want error")
	}
}
