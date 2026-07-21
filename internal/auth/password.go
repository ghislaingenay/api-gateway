package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword returns a bcrypt hash of plaintext suitable for storing in
// User.PasswordHash.
func HashPassword(plaintext string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// ComparePassword reports whether plaintext matches the given bcrypt hash,
// returning ErrInvalidCredentials on mismatch.
func ComparePassword(hash, plaintext string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}
