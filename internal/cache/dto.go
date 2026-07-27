package cache

import "net/http"

// CachedResponse is the stored representation of a downstream response,
// captured and replayed verbatim on a cache hit.
type CachedResponse struct {
	StatusCode int         `json:"status_code"`
	Header     http.Header `json:"header"`
	Body       []byte      `json:"body"`
}
