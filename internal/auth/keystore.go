package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"fmt"

	"api-gateway/config"

	"github.com/golang-jwt/jwt/v5"
)

// KeyStore resolves a signing key by its key ID (kid), allowing multiple
// keys to be active simultaneously so rotation does not reject in-flight
// tokens signed with the previous key.
type KeyStore interface {
	GetKey(kid string) (*rsa.PublicKey, error)
}

type staticKeyStore struct {
	keys map[string]*rsa.PublicKey
}

// NewKeyStore builds a KeyStore from the configured signing keys. Each entry
// in cfg.SigningKeys must be a base64-encoded PEM-encoded RSA public key.
func NewKeyStore(cfg *config.JWTConfig) (KeyStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("jwt config is nil")
	}
	if len(cfg.SigningKeys) == 0 {
		return nil, fmt.Errorf("no signing keys configured")
	}

	keys := make(map[string]*rsa.PublicKey, len(cfg.SigningKeys))
	for kid, encoded := range cfg.SigningKeys {
		pemBytes, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode signing key %q: %w", kid, err)
		}
		key, err := jwt.ParseRSAPublicKeyFromPEM(pemBytes)
		if err != nil {
			return nil, fmt.Errorf("parse signing key %q: %w", kid, err)
		}
		keys[kid] = key
	}
	return &staticKeyStore{keys: keys}, nil
}

// GetKey implements KeyStore.
func (s *staticKeyStore) GetKey(kid string) (*rsa.PublicKey, error) {
	key, ok := s.keys[kid]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownKey, kid)
	}
	return key, nil
}
