package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// x509PublicKeyBytes marshals the RSA public key to DER, simulating an
// attacker using the known public key material as an HMAC secret in an
// algorithm-confusion attack.
func x509PublicKeyBytes(t *testing.T, key *rsa.PublicKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatalf("x509.MarshalPKIXPublicKey() error = %v", err)
	}
	return der
}

func newTestKeyStore(t *testing.T, kid string, key *rsa.PublicKey) KeyStore {
	t.Helper()
	return keyStoreFunc(func(k string) (*rsa.PublicKey, error) {
		if k != kid {
			return nil, ErrUnknownKey
		}
		return key, nil
	})
}

type keyStoreFunc func(kid string) (*rsa.PublicKey, error)

func (f keyStoreFunc) GetKey(kid string) (*rsa.PublicKey, error) { return f(kid) }

func TestJWTAuthMiddleware(t *testing.T) {
	rsaKey := generateRSAKeyPair(t)
	const kid = "kid-1"
	keyStore := newTestKeyStore(t, kid, &rsaKey.PublicKey)

	newProtectedHandler := func(t *testing.T) http.Handler {
		t.Helper()
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				t.Error("expected claims in context, got none")
			}
			if claims != nil && claims.Email != validClaims().Email {
				t.Errorf("unexpected claims in context: %+v", claims)
			}
			w.WriteHeader(http.StatusOK)
		})
	}

	tests := []struct {
		name           string
		authHeader     string
		token          func(t *testing.T) string
		wantStatusCode int
	}{
		{
			name:           "missing authorization header",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "malformed header (no bearer scheme)",
			authHeader:     "not-a-bearer-token",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name: "malformed token (not three segments)",
			token: func(t *testing.T) string {
				return "not.a.validtoken.at.all"
			},
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name: "alg=none rejected",
			token: func(t *testing.T) string {
				return signNone(t, validClaims())
			},
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name: "HMAC/RSA algorithm confusion rejected",
			token: func(t *testing.T) string {
				return signHS256(t, x509PublicKeyBytes(t, &rsaKey.PublicKey), kid, validClaims())
			},
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name: "unknown kid rejected",
			token: func(t *testing.T) string {
				return signRS256(t, rsaKey, "unknown-kid", validClaims())
			},
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name: "expired token rejected",
			token: func(t *testing.T) string {
				c := validClaims()
				c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
				return signRS256(t, rsaKey, kid, c)
			},
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name: "not-yet-valid (nbf) token rejected",
			token: func(t *testing.T) string {
				c := validClaims()
				c.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Hour))
				return signRS256(t, rsaKey, kid, c)
			},
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name: "missing required claim (tenant_id) rejected",
			token: func(t *testing.T) string {
				c := validClaims()
				c.TenantID = uuid.Nil
				return signRS256(t, rsaKey, kid, c)
			},
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name: "valid token accepted and claims attached",
			token: func(t *testing.T) string {
				return signRS256(t, rsaKey, kid, validClaims())
			},
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			middleware := JWTAuthMiddleware(keyStore, []string{"RS256"})
			handler := middleware(newProtectedHandler(t))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			switch {
			case tt.token != nil:
				req.Header.Set("Authorization", "Bearer "+tt.token(t))
			case tt.authHeader != "":
				req.Header.Set("Authorization", tt.authHeader)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatusCode {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatusCode, rec.Body.String())
			}
		})
	}
}

func TestJWTAuthMiddleware_KeyRotation(t *testing.T) {
	t.Parallel()
	oldKey := generateRSAKeyPair(t)
	newKey := generateRSAKeyPair(t)

	multiKeyStore := keyStoreFunc(func(kid string) (*rsa.PublicKey, error) {
		switch kid {
		case "old-kid":
			return &oldKey.PublicKey, nil
		case "new-kid":
			return &newKey.PublicKey, nil
		default:
			return nil, ErrUnknownKey
		}
	})

	middleware := JWTAuthMiddleware(multiKeyStore, []string{"RS256"})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, tc := range []struct {
		kid string
		key *rsa.PrivateKey
	}{
		{"old-kid", oldKey},
		{"new-kid", newKey},
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+signRS256(t, tc.key, tc.kid, validClaims()))

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("kid %q: status = %d, want %d", tc.kid, rec.Code, http.StatusOK)
		}
	}
}
