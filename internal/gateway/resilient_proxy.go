package gateway

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"api-gateway/internal/logger"
	"api-gateway/internal/resilience"
)

// resilientProxier decorates a Proxier with a per-route deadline
// (FEAT-008 FR-2/FR-3) and, for idempotent (GET) requests, exponential
// backoff retries on transient downstream failures (FEAT-008 FR-1). Non-GET
// requests are never retried, but still run under the deadline-bound
// context so a slow downstream call is still aborted with a 504.
type resilientProxier struct {
	next               Proxier
	defaultDeadline    time.Duration
	defaultRetryPolicy resilience.RetryPolicy
}

// NewResilientProxier wraps next with deadline enforcement and GET-only
// retry logic. defaultDeadline and defaultRetryPolicy apply to routes with
// no per-route override.
func NewResilientProxier(next Proxier, defaultDeadline time.Duration, defaultRetryPolicy resilience.RetryPolicy) Proxier {
	return &resilientProxier{
		next:               next,
		defaultDeadline:    defaultDeadline,
		defaultRetryPolicy: defaultRetryPolicy,
	}
}

// Proxy implements Proxier.
func (p *resilientProxier) Proxy(w http.ResponseWriter, r *http.Request, upstream string) {
	deadline := p.defaultDeadline
	policy := p.defaultRetryPolicy

	if route, ok := RouteFromContext(r.Context()); ok && route != nil {
		if route.Deadline > 0 {
			deadline = route.Deadline
		}
		if route.RetryMaxAttempts > 0 {
			policy.MaxAttempts = route.RetryMaxAttempts
		}
	}

	ctx, cancel := resilience.WithDeadline(r.Context(), deadline)
	defer cancel()
	r = r.WithContext(ctx)

	// Only idempotent GET requests are retried (FEAT-008 FR-1 AC4); every
	// other method still gets the deadline-bound context but a single
	// attempt, streamed straight through to the real ResponseWriter.
	if r.Method != http.MethodGet {
		p.next.Proxy(w, r, upstream)
		return
	}

	var bodyBytes []byte
	if r.Body != nil {
		b, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err == nil {
			bodyBytes = b
		}
	}

	for attempt := 1; ; attempt++ {
		if bodyBytes != nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Buffer the response instead of streaming it directly, so a
		// transient failure can be retried before any bytes reach the
		// client.
		rec := newBufferedRecorder()
		p.next.Proxy(rec, r, upstream)

		if !resilience.IsRetryableStatus(rec.statusCode) || attempt >= policy.MaxAttempts {
			rec.flush(w, r.Context())
			return
		}

		logger.FromContext(r.Context()).Info("gateway: retrying downstream call",
			"event_type", "retry_attempt",
			"upstream", upstream,
			"attempt", attempt,
			"status", rec.statusCode,
		)

		select {
		case <-ctx.Done():
			logger.FromContext(r.Context()).Warn("gateway: deadline exceeded during retry backoff",
				"event_type", "timeout",
				"upstream", upstream,
				"attempt", attempt,
			)
			writeError(w, r, http.StatusGatewayTimeout, "gateway_timeout", "downstream service did not respond in time")
			return
		case <-time.After(policy.Backoff(attempt)):
		}
	}
}

// bufferedRecorder captures a downstream response fully in memory instead
// of streaming it to the client immediately, so a retry can be attempted
// before any bytes are committed to the real ResponseWriter (FEAT-008).
type bufferedRecorder struct {
	header      http.Header
	statusCode  int
	wroteHeader bool
	body        bytes.Buffer
}

func newBufferedRecorder() *bufferedRecorder {
	return &bufferedRecorder{header: make(http.Header), statusCode: http.StatusOK}
}

func (r *bufferedRecorder) Header() http.Header {
	return r.header
}

func (r *bufferedRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.statusCode = status
}

func (r *bufferedRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.body.Write(b)
}

// flush replays the buffered response onto w, the response actually sent
// to the client.
func (r *bufferedRecorder) flush(w http.ResponseWriter, ctx context.Context) {
	dst := w.Header()
	for k, v := range r.header {
		dst[k] = v
	}
	w.WriteHeader(r.statusCode)
	if _, err := w.Write(r.body.Bytes()); err != nil {
		logger.FromContext(ctx).Error("gateway: failed to write proxied response", "error", err.Error())
	}
}
