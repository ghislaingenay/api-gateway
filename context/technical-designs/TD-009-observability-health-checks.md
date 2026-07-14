# TD-009: Observability & Health Checks

Status: Draft

Owner: Ghislain Genay
Created: 2026-07-14
Last Updated: 2026-07-14

Feature Spec: [FEAT-009 - Observability & Health Checks](../features/FEAT-009-observability-health-checks.md)

---

# 1. Overview

## Summary

Correlation-ID middleware run first in the chain, a structured JSON logger threaded through every other component via context, and two unauthenticated endpoints (`/health`, `/ready`) for liveness/readiness checks against Redis and PostgreSQL.

## Goals

- Correlation ID generated/propagated on every request, in logs and response headers
- `/health` (liveness) and `/ready` (readiness, dependency-checked) endpoints
- Structured logging integration points for every other feature's key events

## Non-Goals

- Prometheus/Grafana dashboards (metrics exposed, not visualized, in MVP)
- Distributed tracing (W3C Trace Context) — post-MVP

---

# 2. Architecture

## High-Level Design

```
Request
│
▼
CorrelationIDMiddleware (first in chain)
  │ 1. id = header X-Correlation-ID or generate UUID
  │ 2. context.WithValue(ctx, correlationIDKey, id)
  │ 3. response header X-Correlation-ID: id
▼
StructuredLogger (used by all downstream middleware via ctx)
│
▼
[TD-001 → TD-008 all log through ctx-bound logger]

Separately:
GET /health  → 200 (no dependency checks)
GET /ready   → checks Redis PING + Postgres SELECT 1 → 200 or 503
```

---

# 3. Components

## New Components

- `middleware.CorrelationIDMiddleware`
- `log.StructuredLogger` — JSON logger, `log.FromContext(ctx)` accessor used across all features
- `handlers.HealthHandler`, `handlers.ReadyHandler`
- `health.DependencyChecker` — Redis PING, Postgres `SELECT 1`, with short timeouts

## Modified Components

- All prior middleware (TD-001, TD-003, TD-004, TD-005, TD-006, TD-008) — updated to log via `log.FromContext(ctx)` instead of ad hoc logging

---

# 4. Data Model

None — logging and health checks are stateless/read-only against existing infrastructure.

---

# 5. API Design

## New Endpoints

### GET /health

Purpose: liveness probe, no dependency checks.

Response: `200 {"status":"ok"}`

---

### GET /ready

Purpose: readiness probe, checks Redis and PostgreSQL connectivity.

Response: `200 {"status":"ready","redis":"ok","postgres":"ok"}` or `503 {"status":"not_ready","redis":"ok","postgres":"unreachable"}`

---

## Endpoint Changes

All responses (across every route) gain an `X-Correlation-ID` header.

---

# 6. Sequence Flow

```
Request
│
CorrelationIDMiddleware: assign/propagate ID
│
[All subsequent middleware log with ctx-bound correlation ID]
│
Response with X-Correlation-ID header

(Separately, orchestrator polling)
GET /ready
│
Redis PING (timeout-bound) + Postgres SELECT 1 (timeout-bound)
│
Both OK? ──Yes──> 200
│ No
503 with per-dependency status
```

---

# 7. Security

## Authentication

`/health` and `/ready` are explicitly unauthenticated (excluded from TD-001 middleware chain).

## Authorization

N/A — no auth required.

## Data Protection

Health/readiness responses expose only dependency connectivity status, no sensitive data.

## Rate Limiting

`/health` and `/ready` are excluded from rate limiting (TD-005) to avoid orchestrator polling being throttled.

---

# 8. Performance

## Expected Load

`/ready` polled frequently by orchestration (e.g., every few seconds); must be fast and cheap.

## Database Impact

`/ready` issues one lightweight `SELECT 1` per poll — negligible load.

## Caching Strategy

N/A — freshness matters more than performance here; no caching of readiness state.

---

# 9. Monitoring

## Metrics

- `http_requests_total{route,status,event_type}` — includes tagged events like `auth_failure`, `authz_deny`, `rate_limit_fail_open`, `cache_hit`, `cache_miss`, `retry_attempt`, `timeout`
- `ready_check_duration_seconds{dependency}`

## Logging

- Every request logs at minimum: correlation ID, method, path, status, duration, tenant_id (if resolved)
- All other features log through this shared logger

## Alerts

- `/ready` returning 503 for a sustained period (dependency outage)
- Log pipeline ingestion failure (meta-alert, if using external aggregation)

---

# 10. Risks

## Risk 1

`/ready` check itself hanging if a dependency is unresponsive rather than erroring.

Mitigation: bound each dependency check with a short timeout (e.g., 1-2s) so `/ready` always responds promptly even under dependency degradation.

---

## Risk 2

Log volume becoming unmanageable under high load.

Mitigation: structured JSON logs are aggregation-friendly by design; log sampling deferred as a post-MVP optimization if needed.

---

# 11. Rollout Plan

## Deployment

1. Deploy correlation ID + structured logging first (foundational, alongside TD-001)
2. Deploy `/health` and `/ready` endpoints
3. Wire orchestration (Docker Compose healthcheck, per TD-010) to poll `/ready`

## Rollback

1. Revert to unstructured logging temporarily if logger integration breaks (not expected, but a safe fallback)
2. Roll back deployment

---

# 12. Open Questions

- Log sampling strategy at high volume (deferred, likely post-MVP)

---

# 13. ADR References

- ADR-004: Use Go for API Gateway Implementation
