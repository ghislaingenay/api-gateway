// Package clientip extracts a caller's address for keying rate limits and
// failure counters before any authenticated identity exists. It has no
// internal dependencies so both internal/auth and internal/ratelimit can
// import it without introducing a cycle.
package clientip

import (
	"net"
	"net/http"
	"strings"
)

// FromRequest returns the caller's address: the first hop of
// X-Forwarded-For if present, otherwise the host portion of r.RemoteAddr.
func FromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := strings.TrimSpace(strings.Split(xff, ",")[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
