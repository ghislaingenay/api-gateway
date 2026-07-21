package auth

import "testing"

func TestHashPassword_ComparePassword(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if err := ComparePassword(hash, "correct-horse-battery-staple"); err != nil {
		t.Errorf("ComparePassword() with correct password error = %v, want nil", err)
	}

	if err := ComparePassword(hash, "wrong-password"); err == nil {
		t.Error("ComparePassword() with wrong password error = nil, want ErrInvalidCredentials")
	}
}

func TestHashPassword_NonDeterministic(t *testing.T) {
	t.Parallel()

	hash1, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	hash2, err := HashPassword("same-password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if hash1 == hash2 {
		t.Error("HashPassword() produced identical hashes for two calls with the same input, want distinct salts")
	}

	if err := ComparePassword(hash1, "same-password"); err != nil {
		t.Errorf("ComparePassword(hash1) error = %v, want nil", err)
	}
	if err := ComparePassword(hash2, "same-password"); err != nil {
		t.Errorf("ComparePassword(hash2) error = %v, want nil", err)
	}
}
