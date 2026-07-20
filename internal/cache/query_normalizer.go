package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

// NormalizeQueryHash returns a canonical hash of a raw query string: params
// are sorted by key, and multi-valued params are sorted by value, so
// requests that differ only in query parameter order hash to the same key
// (FEAT-006 Edge Cases) instead of producing spurious cache misses. A
// malformed query string hashes the raw string directly rather than falling
// back to the empty-query hash, so it never collides with a request that
// genuinely has no query string.
func NormalizeQueryHash(rawQuery string) string {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		sum := sha256.Sum256([]byte("malformed:" + rawQuery))
		return hex.EncodeToString(sum[:])
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		vals := append([]string(nil), values[k]...)
		sort.Strings(vals)
		for _, v := range vals {
			b.WriteString(k)
			b.WriteByte('=')
			b.WriteString(v)
			b.WriteByte('&')
		}
	}

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
