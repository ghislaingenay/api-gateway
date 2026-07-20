# Current Feature

FEAT-005: Distributed Rate Limiting

## File

[FEAT-005-distributed-rate-limiting.md](features/FEAT-005-distributed-rate-limiting.md)

## Goals

- [ ] FR-1: Check and increment a per-tenant sliding-window counter in Redis before routing each request
- [ ] FR-2: Rate limits default from environment/config, overridable per tenant via `tenants` table
- [ ] FR-3: Fail open (allow the request) and emit an alertable log if Redis is unavailable

## Notes

Redis-backed sliding-window (two-bucket approximation) rate limiter, per tenant+user,
enforcing per-minute and per-hour windows. Builds on FEAT-004 (tenant_id resolution,
auth.ClaimsFromContext) and the existing `internal/tenant` Redis-backed cache pattern.
`tenants.rate_limit_per_minute`/`rate_limit_per_hour` columns already exist (FEAT-004
migration) — no schema change needed. Metrics requirement satisfied via structured
`log.Printf` output (no Prometheus infra in repo yet) per user decision; FEAT-008
(Observability) is still Draft but proceeding anyway per user decision.
