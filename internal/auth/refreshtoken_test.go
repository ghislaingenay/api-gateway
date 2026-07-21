package auth

import "testing"

func TestGenerateRefreshToken_Unique(t *testing.T) {
	t.Parallel()

	raw1, hash1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}
	raw2, hash2, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}

	if raw1 == raw2 {
		t.Error("GenerateRefreshToken() produced identical raw tokens across two calls")
	}
	if hash1 == hash2 {
		t.Error("GenerateRefreshToken() produced identical hashes across two calls")
	}
	if len(hash1) != 64 {
		t.Errorf("len(hash) = %d, want 64 (sha256 hex)", len(hash1))
	}
}

func TestHashRefreshToken_Deterministic(t *testing.T) {
	t.Parallel()

	raw, wantHash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}

	if got := HashRefreshToken(raw); got != wantHash {
		t.Errorf("HashRefreshToken(raw) = %q, want %q", got, wantHash)
	}
	if HashRefreshToken("other-value") == wantHash {
		t.Error("HashRefreshToken() produced the same hash for a different input")
	}
}
