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

// maxLen bounds the returned value so a caller can't inflate Redis key size
// (or memory) by sending an oversized X-Forwarded-For header.
const maxLen = 64

// FromRequest returns the caller's address for rate-limiting purposes.
// trustedHops counts trusted reverse proxies in front of this service
// (e.g. 1 for a single load balancer); <= 0 ignores X-Forwarded-For
// entirely and trusts only r.RemoteAddr, since the header is otherwise
// attacker-controlled. When trustedHops > 0, the value read is that many
// entries from the *right* of the header — proxies append, they don't
// overwrite, so the leftmost entry can be forged by the client itself.
// Requires the service to be unreachable except through those hops.
func FromRequest(r *http.Request, trustedHops int) string {
	if trustedHops > 0 {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			idx := len(parts) - trustedHops
			if idx >= 0 && idx < len(parts) {
				if ip := strings.TrimSpace(parts[idx]); ip != "" {
					return truncate(ip)
				}
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return truncate(r.RemoteAddr)
	}
	return truncate(host)
}

func truncate(s string) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}
