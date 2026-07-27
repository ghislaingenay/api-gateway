package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"api-gateway/config"

	"github.com/MicahParks/jwkset"
)

func TestNewKeyStore_And_GetKey(t *testing.T) {
	t.Parallel()

	key1 := generateRSAKeyPair(t)
	key2 := generateRSAKeyPair(t)
	srv, _ := newJWKSServer(t, map[string]*rsa.PublicKey{
		"kid-1": &key1.PublicKey,
		"kid-2": &key2.PublicKey,
	})

	cfg := &config.JWTConfig{
		JWKSURL:             srv.URL,
		JWKSRefreshInterval: time.Hour,
	}

	store, err := newKeyStore(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewKeyStore() error = %v", err)
	}

	if _, err := store.GetKey("kid-1"); err != nil {
		t.Errorf("GetKey(kid-1) error = %v, want nil", err)
	}
	if _, err := store.GetKey("kid-2"); err != nil {
		t.Errorf("GetKey(kid-2) error = %v, want nil", err)
	}

	_, err = store.GetKey("unknown-kid")
	if !errors.Is(err, ErrUnknownKey) {
		t.Errorf("GetKey(unknown-kid) error = %v, want ErrUnknownKey", err)
	}
}

func TestNewKeyStore_NoJWKSURL(t *testing.T) {
	t.Parallel()

	if _, err := NewKeyStore(&config.JWTConfig{}); err == nil {
		t.Error("NewKeyStore() error = nil, want error for empty JWKSURL")
	}
}

func TestNewKeyStore_NilConfig(t *testing.T) {
	t.Parallel()

	if _, err := NewKeyStore(nil); err == nil {
		t.Error("NewKeyStore() error = nil, want error for nil config")
	}
}

func TestNewKeyStore_UnreachableEndpoint(t *testing.T) {
	t.Parallel()

	key := generateRSAKeyPair(t)
	srv, _ := newJWKSServer(t, map[string]*rsa.PublicKey{"kid-1": &key.PublicKey})
	unreachableURL := srv.URL
	srv.Close() // endpoint is now unreachable

	if _, err := NewKeyStore(&config.JWTConfig{JWKSURL: unreachableURL}); err == nil {
		t.Error("NewKeyStore() error = nil, want error for unreachable JWKS endpoint")
	}
}

func TestJWKSKeyStore_NonRSAKeyRejected(t *testing.T) {
	t.Parallel()

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() error = %v", err)
	}
	jwk, err := jwkset.NewJWKFromKey(&ecKey.PublicKey, jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{KID: "ec-kid", ALG: jwkset.AlgES256},
	})
	if err != nil {
		t.Fatalf("jwkset.NewJWKFromKey() error = %v", err)
	}
	body, err := json.Marshal(jwkset.JWKSMarshal{Keys: []jwkset.JWKMarshal{jwk.Marshal()}})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	store, err := newKeyStore(t.Context(), &config.JWTConfig{JWKSURL: srv.URL})
	if err != nil {
		t.Fatalf("NewKeyStore() error = %v", err)
	}

	if _, err := store.GetKey("ec-kid"); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("GetKey(ec-kid) error = %v, want ErrUnknownKey (non-RSA key rejected)", err)
	}
}

func TestJWKSKeyStore_RefreshFailurePreservesStaleKeys(t *testing.T) {
	t.Parallel()

	key := generateRSAKeyPair(t)
	srv, _ := newJWKSServer(t, map[string]*rsa.PublicKey{"kid-1": &key.PublicKey})

	const refreshInterval = 20 * time.Millisecond
	store, err := newKeyStore(t.Context(), &config.JWTConfig{
		JWKSURL:             srv.URL,
		JWKSRefreshInterval: refreshInterval,
	})
	if err != nil {
		t.Fatalf("NewKeyStore() error = %v", err)
	}

	if _, err := store.GetKey("kid-1"); err != nil {
		t.Fatalf("GetKey(kid-1) before outage error = %v, want nil", err)
	}

	srv.Close() // simulate the JWKS endpoint going down mid-life

	// Give the background refresh goroutine a few cycles to hit the now-dead
	// endpoint and fail.
	time.Sleep(5 * refreshInterval)

	if _, err := store.GetKey("kid-1"); err != nil {
		t.Errorf("GetKey(kid-1) after failed background refresh error = %v, want nil (stale key must remain usable)", err)
	}
}

func TestJWKSKeyStore_Rotation(t *testing.T) {
	t.Parallel()

	key1 := generateRSAKeyPair(t)
	key2 := generateRSAKeyPair(t)
	srv, setKeys := newJWKSServer(t, map[string]*rsa.PublicKey{"kid-1": &key1.PublicKey})

	const refreshInterval = 20 * time.Millisecond
	store, err := newKeyStore(t.Context(), &config.JWTConfig{
		JWKSURL:             srv.URL,
		JWKSRefreshInterval: refreshInterval,
	})
	if err != nil {
		t.Fatalf("NewKeyStore() error = %v", err)
	}

	if _, err := store.GetKey("kid-1"); err != nil {
		t.Fatalf("GetKey(kid-1) before rotation error = %v, want nil", err)
	}
	if _, err := store.GetKey("kid-2"); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("GetKey(kid-2) before rotation error = %v, want ErrUnknownKey", err)
	}

	setKeys(map[string]*rsa.PublicKey{"kid-2": &key2.PublicKey})

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := store.GetKey("kid-2"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("GetKey(kid-2) never became resolvable after rotation within deadline")
		}
		time.Sleep(refreshInterval)
	}
}
