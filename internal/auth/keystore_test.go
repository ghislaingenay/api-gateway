package auth

import (
	"errors"
	"testing"

	"api-gateway/config"
)

func TestNewKeyStore_And_GetKey(t *testing.T) {
	key1 := generateRSAKeyPair(t)
	key2 := generateRSAKeyPair(t)

	cfg := &config.JWTConfig{
		SigningKeys: map[string]string{
			"kid-1": encodePublicKeyPEM(t, &key1.PublicKey),
			"kid-2": encodePublicKeyPEM(t, &key2.PublicKey),
		},
	}

	store, err := NewKeyStore(cfg)
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

func TestNewKeyStore_InvalidKey(t *testing.T) {
	cfg := &config.JWTConfig{
		SigningKeys: map[string]string{
			"kid-1": "not-valid-base64!!!",
		},
	}

	if _, err := NewKeyStore(cfg); err == nil {
		t.Error("NewKeyStore() error = nil, want error for invalid base64")
	}
}
