package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// Signer mints signed JWTs for validated claims.
type Signer interface {
	Sign(claims CustomClaims) (string, error)
}

type rsaSigner struct {
	kid        string
	privateKey *rsa.PrivateKey
}

// NewSigner builds a Signer from a key ID and a base64-encoded PEM-encoded
// RSA private key. The public half of this key must be present in the
// KeyStore under the same kid so tokens this Signer mints can be verified
// by JWTAuthMiddleware.
func NewSigner(kid, privateKeyPEMBase64 string) (Signer, error) {
	if kid == "" {
		return nil, fmt.Errorf("signer: kid is required")
	}
	pemBytes, err := base64.StdEncoding.DecodeString(privateKeyPEMBase64)
	if err != nil {
		return nil, fmt.Errorf("signer: decode private key: %w", err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("signer: parse private key: %w", err)
	}
	return &rsaSigner{kid: kid, privateKey: key}, nil
}

// Sign implements Signer.
func (s *rsaSigner) Sign(claims CustomClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.kid
	signed, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", fmt.Errorf("signer: sign token: %w", err)
	}
	return signed, nil
}
