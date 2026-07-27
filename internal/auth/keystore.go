package auth

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"

	"api-gateway/config"
	"api-gateway/internal/logger"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
)

// KeyStore resolves a signing key by its key ID (kid), allowing multiple
// keys to be active simultaneously so rotation does not reject in-flight
// tokens signed with the previous key.
type KeyStore interface {
	GetKey(kid string) (*rsa.PublicKey, error)
}

// jwksKeyStore resolves keys from a remote JWKS endpoint, refreshed in the
// background so newly rotated keys become active without a gateway
// restart.
type jwksKeyStore struct {
	kf     keyfunc.Keyfunc
	cancel context.CancelFunc
}

// NewKeyStore builds a KeyStore backed by the JWKS endpoint configured in
// cfg.JWKSURL. It fetches the initial key set synchronously and returns an
// error if cfg.JWKSURL is empty or that initial fetch fails, so the gateway
// fails fast at startup rather than serving traffic with no usable keys.
// Once built, the store refreshes its key set in the background every
// cfg.JWKSRefreshInterval; refresh failures are logged and the previously
// fetched keys stay usable until the next successful refresh. The
// background refresh goroutine runs for the lifetime of the process; call
// Close on the returned store to stop it early (e.g. in tests).
func NewKeyStore(cfg *config.JWTConfig) (KeyStore, error) {
	return newKeyStore(context.Background(), cfg)
}

func newKeyStore(ctx context.Context, cfg *config.JWTConfig) (*jwksKeyStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("jwt config is nil")
	}
	if cfg.JWKSURL == "" {
		return nil, fmt.Errorf("no JWKS URL configured")
	}

	refreshCtx, cancel := context.WithCancel(ctx)

	storage, err := jwkset.NewStorageFromHTTP(cfg.JWKSURL, jwkset.HTTPClientStorageOptions{
		Ctx:             refreshCtx,
		RefreshInterval: cfg.JWKSRefreshInterval,
		RefreshErrorHandler: func(_ context.Context, err error) {
			logger.Default().Error("jwks: background refresh failed, keeping last-known-good keys", "url", cfg.JWKSURL, "error", err.Error())
		},
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("fetch initial JWKS from %q: %w", cfg.JWKSURL, err)
	}

	kf, err := keyfunc.New(keyfunc.Options{Storage: storage})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("build JWKS key store: %w", err)
	}

	return &jwksKeyStore{kf: kf, cancel: cancel}, nil
}

// Close stops the background refresh goroutine. Safe to call more than
// once.
func (s *jwksKeyStore) Close() {
	s.cancel()
}

// GetKey implements KeyStore.
func (s *jwksKeyStore) GetKey(kid string) (*rsa.PublicKey, error) {
	jwk, err := s.kf.Storage().KeyRead(context.Background(), kid)
	if err != nil {
		if errors.Is(err, jwkset.ErrKeyNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrUnknownKey, kid)
		}
		return nil, fmt.Errorf("read key %q from JWKS storage: %w", kid, err)
	}

	key, ok := jwk.Key().(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: %s is not an RSA key", ErrUnknownKey, kid)
	}
	return key, nil
}
