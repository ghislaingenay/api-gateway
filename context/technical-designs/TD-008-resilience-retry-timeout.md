# TD-008: Resilience (Retry & Timeout)

Status: Done

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-22

Feature Spec: [FEAT-008 - Resilience (Retry & Timeout)](../features/FEAT-008-resilience-retry-timeout.md)

---

# 1. Overview

## Summary

A wrapper around the downstream proxy call (TD-004) that enforces a deadline-bound `context.Context`, retries idempotent (GET) requests on transient failures with exponential backoff and jitter, and never exceeds the overall request deadline.

## Goals

- Deadline propagation from client request through to downstream call
- Exponential backoff with jitter for GET retries
- Total retry time bounded by remaining deadline budget

## Non-Goals

- Circuit breakers (post-MVP)

---

# 2. Architecture

## High-Level Design

```
Routing decision made (TD-004)
│
▼
ResilientProxyCall
  │ 1. ctx, cancel = context.WithTimeout(parentCtx, routeDeadline)
  │ 2. attempt = 0
  │ 3. loop: call downstream with ctx
  │      transient failure + idempotent + attempts < max + ctx not done?
  │        → backoff (exponential + jitter) → retry
  │      else → return result/error
  │ 4. ctx.Done() at any point → abort, return 504
▼
Response (or 504 Gateway Timeout)
```

---

# 3. Components

## New Components

- `resilience.RetryPolicy` — max attempts, base backoff, jitter, retryable-status set
- `resilience.WithDeadline(parentCtx, routeConfig) (context.Context, context.CancelFunc)`
- `proxy.ResilientCall` — wraps `proxy.ReverseProxy` (TD-004) with retry/timeout logic

## Modified Components

- `router.Route` config (TD-004) — extended with `Deadline`, `RetryPolicy` (idempotent-only) fields

---

# 4. Data Model

None — stateless, in-request logic only.

---

# 5. API Design

## New Endpoints

None.

## Endpoint Changes

Downstream timeout now surfaces as:

```json
504 { "error": "gateway_timeout", "message": "downstream service did not respond in time" }
```

---

# 6. Sequence Flow

```
Downstream call needed (routing resolved)
│
Derive deadline-bound context
│
Attempt downstream call
│
Success (2xx-4xx, non-retryable) → return response
│
Transient failure (5xx/conn error) + idempotent + attempts remaining + time remaining?
  Yes → backoff (exp + jitter) → retry
  No  → return error / 504
│
ctx.Done() at any point → abort → 504
```

---

# 7. Security

## Authentication

N/A (runs after TD-001/TD-003).

## Authorization

N/A.

## Data Protection

Retries only apply to idempotent (GET) requests to avoid duplicate side effects on non-idempotent operations.

## Rate Limiting

Retries consume additional downstream capacity but do not re-check gateway rate limits (already checked once per client request in TD-005) — TODO: confirm whether retries should count against downstream-specific limits, not specified in overview.

---

# 8. Performance

## Expected Load

Retries add latency only on transient failure paths; healthy-path latency unaffected.

## Database Impact

None.

## Caching Strategy

N/A (interacts with TD-006 caching only in that a successful retried response can still be cached on 2xx).

---

# 9. Monitoring

## Metrics

- `downstream_retry_attempts_total{route}`
- `downstream_timeout_total{route}`

## Logging

- Every retry attempt logged with correlation ID, attempt number, reason
- Deadline exceeded events logged distinctly

## Alerts

- Elevated retry rate for a specific downstream service (indicates degraded service)
- Elevated timeout rate (indicates service is failing, not just transiently slow)

---

# 10. Risks

## Risk 1

Retry storms amplifying load on an already-struggling downstream service.

Mitigation: exponential backoff with jitter spreads retries out; bounded max attempts; full circuit breaker deferred to post-MVP as the more complete mitigation.

---

## Risk 2

Retrying a non-idempotent request by mistake, causing duplicate side effects.

Mitigation: retry policy explicitly scoped to GET (and any route explicitly marked idempotent-safe) only, enforced at route configuration level.

---

# 11. Rollout Plan

## Deployment

1. Deploy alongside TD-004 (wraps the same proxy call)
2. Configure default deadline and retry policy per route
3. Verify via chaos-style integration tests (inject transient downstream failures)

## Rollback

1. Disable retry logic via feature flag (single-attempt calls with deadline still enforced)
2. Roll back deployment

---

# 12. Open Questions

- Default deadline and max retry attempt values
- Whether retries should be counted against a separate downstream-specific rate budget

---

# 13. ADR References

- ADR-004: Use Go for API Gateway Implementation
