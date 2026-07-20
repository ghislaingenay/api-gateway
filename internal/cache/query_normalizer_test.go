package cache

import "testing"

func TestNormalizeQueryHash(t *testing.T) {
	t.Parallel()

	t.Run("param order does not affect hash", func(t *testing.T) {
		t.Parallel()
		a := NormalizeQueryHash("b=2&a=1")
		b := NormalizeQueryHash("a=1&b=2")
		if a != b {
			t.Fatalf("hashes differ for reordered params: %q vs %q", a, b)
		}
	})

	t.Run("different values produce different hashes", func(t *testing.T) {
		t.Parallel()
		a := NormalizeQueryHash("a=1")
		b := NormalizeQueryHash("a=2")
		if a == b {
			t.Fatalf("expected different hashes for different values, got %q", a)
		}
	})

	t.Run("malformed query does not collide with an empty query", func(t *testing.T) {
		t.Parallel()
		empty := NormalizeQueryHash("")
		malformed := NormalizeQueryHash("%zz")
		if empty == malformed {
			t.Fatalf("expected malformed query to hash differently from an empty query, both got %q", empty)
		}
	})

	t.Run("malformed queries are stable and distinguish different raw input", func(t *testing.T) {
		t.Parallel()
		a := NormalizeQueryHash("%zz")
		b := NormalizeQueryHash("%zz")
		c := NormalizeQueryHash("%yy")
		if a != b {
			t.Fatalf("expected the same malformed query to hash consistently, got %q vs %q", a, b)
		}
		if a == c {
			t.Fatalf("expected different malformed queries to hash differently, both got %q", a)
		}
	})

	t.Run("repeated keys with reordered values match", func(t *testing.T) {
		t.Parallel()
		a := NormalizeQueryHash("tag=b&tag=a")
		b := NormalizeQueryHash("tag=a&tag=b")
		if a != b {
			t.Fatalf("hashes differ for reordered repeated-key values: %q vs %q", a, b)
		}
	})
}
