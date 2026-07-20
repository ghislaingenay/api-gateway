# Wire RateLimitMiddleware into internal/server (FEAT-005)

## Context

FEAT-005 (Distributed Rate Limiting) adds
`ratelimit.RateLimitMiddleware(limiter ratelimit.Limiter, limits
ratelimit.LimitsProvider, defaults ratelimit.Defaults) func(http.Handler)
http.Handler`, which needs to run in `internal/server` between
`auth.JWTAuthMiddleware` and `gateway.NewHandler` on the `/api/` route (it
reads `tenant_id`/`user_id` from validated claims, same as the gateway
handler).

## Why this shape

`internal/server/server.go`'s `Server` struct already threads every
middleware/handler dependency the same way: raw constructor inputs are
stored as fields on `Server` in `NewServer`, then composed together inside
`RegisterRoutes` (`internal/server/routes.go`). For example:

- `keyStore` + `jwtAlgorithms` are stored raw, then
  `auth.JWTAuthMiddleware(s.keyStore, s.jwtAlgorithms)` is called at
  registration time.
- `routeTable`, `tenantStatus`, `proxy` are stored raw, then passed into
  `gateway.NewHandler(s.routeTable, s.tenantStatus, s.proxy)`.

`RateLimitMiddleware` takes three constructor inputs (limiter, limits
provider, defaults), so the same convention adds three fields —
`rateLimiter ratelimit.Limiter`, `rateLimits ratelimit.LimitsProvider`,
`rateLimitDefs ratelimit.Defaults` — rather than pre-building the
`func(http.Handler) http.Handler` closure inside `NewServer` and storing
just that. Keeping raw inputs on the struct matches every existing
integration in this file and keeps `RegisterRoutes` the single place that
composes the middleware chain, instead of splitting composition between two
places.

## Alternative considered (not taken now)

Build the finished middleware closure once in `NewServer` and store a
single `rateLimit func(http.Handler) http.Handler` field instead of three.
This shrinks the `Server` struct by two fields for this feature, but
diverges from how every other piece of middleware in this file is wired
(auth and gateway both store raw inputs, not pre-built closures) — adopting
it here alone would leave the file with two inconsistent patterns side by
side.

If `Server`'s field count becomes a real problem as more middleware is
added, revisit this for the whole file at once (auth + gateway + rate
limit), not as a one-off for rate limiting.

## Changes

- `internal/server/server.go`: import `internal/ratelimit`; add
  `rateLimiter`, `rateLimits`, `rateLimitDefs` fields to `Server`; in
  `NewServer`, build a `ratelimit.SlidingWindowLimiter` from the existing
  `*redis.Client`, reuse the existing `tenant.NewStatusCache(...)` result as
  the `LimitsProvider` (it already satisfies `RateLimits`), and load
  `ratelimit.Defaults` from `config.LoadRateLimitConfig()`.
- `internal/server/routes.go`: wrap `gateway.NewHandler(...)` with
  `ratelimit.RateLimitMiddleware(s.rateLimiter, s.rateLimits,
  s.rateLimitDefs)`, inside `auth.JWTAuthMiddleware(...)` and outside the
  gateway handler, on the `/api/` route.

## Out of scope

- Prometheus/metrics infrastructure — FR-3's "monitorable metric" is
  satisfied via structured `log.Printf` output for this MVP (no metrics
  library exists in the repo today), per explicit decision when this
  feature was scoped.
- FEAT-008 (Observability & Health Checks) integration — FEAT-005 proceeds
  independently since FEAT-008 is still Draft.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` pass.
- Manual `curl` against `/api/...` with a valid JWT: confirm
  `X-RateLimit-Limit`/`X-RateLimit-Remaining` headers on allowed requests,
  a 429 with `Retry-After` once the per-minute limit is exceeded, and that
  stopping Redis causes requests to still succeed (fail open) with a
  `ratelimit: ... failing open` log line.
