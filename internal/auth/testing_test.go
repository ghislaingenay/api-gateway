package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// generateRSAKeyPair returns a fresh RSA key pair for signing test tokens.
func generateRSAKeyPair(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() error = %v", err)
	}
	return key
}

// encodePublicKeyPEM base64-encodes the PEM-encoded PKIX public key, matching
// the format config.JWTConfig.SigningKeys expects.
func encodePublicKeyPEM(t *testing.T, key *rsa.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	block := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	return base64.StdEncoding.EncodeToString(block)
}

// validClaims returns CustomClaims that pass Validate() and standard
// registered-claim checks (not expired, already valid).
func validClaims() CustomClaims {
	now := time.Now()
	return CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
		},
		TenantID: uuid.New(),
		UserID:   uuid.New(),
		Role:     "admin",
		RoleID:   uuid.New(),
		Email:    "user@example.com",
	}
}

// signRS256 signs claims with key under kid, using RS256.
func signRS256(t *testing.T, key *rsa.PrivateKey, kid string, claims CustomClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return signed
}

// signHS256 signs claims with an HMAC secret, used to simulate an
// RSA/HMAC algorithm-confusion attack attempt.
func signHS256(t *testing.T, secret []byte, kid string, claims CustomClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return signed
}

// signNone builds an alg=none token with no signature.
func signNone(t *testing.T, claims CustomClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return signed
}
