package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
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

// encodePrivateKeyPEM base64-encodes the PEM-encoded PKCS1 private key,
// matching the format config.JWTConfig.SigningPrivateKey expects.
func encodePrivateKeyPEM(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	der := x509.MarshalPKCS1PrivateKey(key)
	block := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	return base64.StdEncoding.EncodeToString(block)
}

// validClaims returns CustomClaims that pass Validate() and standard
// registered-claim checks (not expired, already valid).
func validClaims() CustomClaims {
	now := time.Now()
	return CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "keycloak-sub-" + uuidLikeSuffix(),
			Issuer:    testIssuer,
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
			NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
		},
		Email: "user@example.com",
	}
}

// testIssuer is the issuer validClaims stamps and JWTAuthMiddleware in
// tests is configured to require.
const testIssuer = "https://keycloak.test/realms/api-gateway"

// uuidLikeSuffix returns a short unique suffix so tests generating multiple
// validClaims don't collide on Subject.
func uuidLikeSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
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

// jwkSetJSON encodes the given kid->public-key pairs as a JWKS JSON
// document, the format served by a real JWKS endpoint.
func jwkSetJSON(t *testing.T, keys map[string]*rsa.PublicKey) []byte {
	t.Helper()
	marshal := jwkset.JWKSMarshal{Keys: make([]jwkset.JWKMarshal, 0, len(keys))}
	for kid, key := range keys {
		jwk, err := jwkset.NewJWKFromKey(key, jwkset.JWKOptions{
			Metadata: jwkset.JWKMetadataOptions{KID: kid, ALG: jwkset.AlgRS256},
		})
		if err != nil {
			t.Fatalf("jwkset.NewJWKFromKey() error = %v", err)
		}
		marshal.Keys = append(marshal.Keys, jwk.Marshal())
	}
	body, err := json.Marshal(marshal)
	if err != nil {
		t.Fatalf("json.Marshal(JWKSMarshal) error = %v", err)
	}
	return body
}

// newJWKSServer starts an httptest.Server serving the given kid->public-key
// pairs as a JWKS JSON document. The returned setKeys func lets a test swap
// the served key set at runtime, to exercise the store's background
// refresh (rotation) behavior.
func newJWKSServer(t *testing.T, keys map[string]*rsa.PublicKey) (server *httptest.Server, setKeys func(map[string]*rsa.PublicKey)) {
	t.Helper()

	var mu sync.Mutex
	body := jwkSetJSON(t, keys)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	setKeys = func(newKeys map[string]*rsa.PublicKey) {
		newBody := jwkSetJSON(t, newKeys)
		mu.Lock()
		defer mu.Unlock()
		body = newBody
	}
	return srv, setKeys
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
