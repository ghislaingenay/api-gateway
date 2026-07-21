package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// refreshTokenBytes is the amount of entropy in a generated refresh token.
const refreshTokenBytes = 32

// GenerateRefreshToken returns a fresh opaque refresh token: raw is the
// value handed to the client, hash is its SHA-256 hex digest for storage
// (the raw value itself is never persisted).
func GenerateRefreshToken() (raw string, hash string, err error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashRefreshToken(raw), nil
}

// HashRefreshToken returns the SHA-256 hex digest of a raw refresh token,
// used to look it up without ever storing the raw value.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
