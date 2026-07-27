# TD-011: JWKS Key Rotation

Status: Done

Owner: Ghislain Genay
Created: 2026-07-27
Last Updated: 2026-07-27

Feature Spec: [FEAT-011 - JWKS Key Rotation](../features/FEAT-011-jwks-key-rotation.md)

---

# 1. Overview

## Summary

Replace `internal/auth`'s static, PEM-in-env-var `staticKeyStore` with a
JWKS-backed `jwksKeyStore` built on `github.com/MicahParks/keyfunc/v3` (and
its underlying `github.com/MicahParks/jwkset`), the library pairing designed
for `golang-jwt/jwt/v5` already in use. The gateway fetches public keys from
a `JWT_JWKS_URL`, caches them in memory, and refreshes them on a background
interval — new keys become resolvable without a restart. This change is
scoped to `internal/auth` + `config` only; `middleware.go`, `claims.go`,
`context.go`, and `errors.go` are untouched because `KeyStore` is already a
narrow interface (`GetKey(kid string) (*rsa.PublicKey, error)`).

## Goals

- `NewKeyStore(cfg *config.JWTConfig) (KeyStore, error)` always builds a
  JWKS-backed store; signature is unchanged so callers don't change
- Background refresh tied to `cfg.JWKSRefreshInterval`, with refresh
  failures logged (not fatal) so stale keys stay usable
- `GetKey` keeps mapping "not found" to the existing `ErrUnknownKey`
  sentinel so `middleware.go`'s 401 handling keeps working unchanged

## Non-Goals

- Static-key fallback/dev-mode path
- Building the JWKS-publishing side (auth/identity server's responsibility)
- Wiring `JWTAuthMiddleware`/`NewKeyStore` into `cmd/api/main.go` or
  `internal/server` routing (already unwired today, separate follow-up)

---

# 2. Architecture

## High-Level Design

```
Auth/Identity Server
│ publishes public keys
▼
GET {JWT_JWKS_URL}  (JSON Web Key Set)
│
jwksKeyStore  (internal/auth/keystore.go)
  │ keyfunc/jwkset-backed jwkset.Storage, built once in NewKeyStore
  │ background refresh goroutine every cfg.JWKSRefreshInterval
  │ refresh-error handler logs failures without crashing the process;
  │ stale keys remain usable until the next successful refresh
▼
GetKey(kid string) (*rsa.PublicKey, error)
  │ type-asserts to *rsa.PublicKey (rejects non-RSA keys)
  │ unknown kid → ErrUnknownKey
▼
JWTAuthMiddleware (internal/auth/middleware.go — unchanged, TD-001)
```

---

# 3. Components

## New Components

- `auth.jwksKeyStore` (`internal/auth/keystore.go`) — wraps a
  `jwkset.Storage` obtained via `keyfunc`/`jwkset`'s HTTP-client
  constructor pointed at `cfg.JWKSURL`
- JWKS-encoding test helper in `internal/auth/testing_test.go` — encodes an
  `*rsa.PublicKey` as a JWK (base64url `n`/`e`) for `httptest.Server`
  fixtures, reusing the existing `generateRSAKeyPair` helper

## Modified Components

- `config/jwt.go` — `LoadJWTConfig`: remove `SigningKeys` field and all
  `JWT_SIGNING_KEYS` parsing; add `JWKSURL string` (required,
  `JWT_JWKS_URL`) and `JWKSRefreshInterval time.Duration` (optional,
  `JWT_JWKS_REFRESH_INTERVAL`, parsed via `time.ParseDuration`, defaults to
  1h if unset/invalid). `AllowedAlgorithms`/`JWT_ALLOWED_ALGORITHMS` parsing
  unchanged.
- `internal/auth/keystore.go` — delete `staticKeyStore` and its PEM-decoding
  logic; `NewKeyStore` now always builds a `jwksKeyStore`, erroring if
  `cfg.JWKSURL` is empty or the initial fetch fails (fail fast at startup)
- `internal/auth/keystore_test.go` — static-store test cases replaced with
  JWKS-store tests against an `httptest.Server`
- `internal/auth/middleware_test.go` — update however it constructs a
  `KeyStore` for test fixtures to use the new JWKS test helper; no
  assertion/logic changes expected
- `context/technical-designs/TD-001-jwt-authentication.md` — key management
  Open Question updated to record the JWKS decision, superseding "No
  KMS/JWKS integration for MVP," and note the new env vars

---

# 4. Data Model

## New Tables

None — key material is fetched over HTTP and cached in memory; no new
PostgreSQL tables.

## Schema Changes

None.

## Redis Keys

None — the JWKS cache lives in the `jwkset.Storage` instance's in-process
memory, not Redis.

---

# 5. API Design

## New Endpoints

None — this is a library-level change internal to the gateway process; no
new gateway-facing HTTP endpoints.

## Endpoint Changes

None — `middleware.go`'s 401 response shape (TD-001) is unchanged; only the
key-resolution mechanism behind it changes.

---

# 6. Sequence Flow

```
Gateway startup
│
NewKeyStore(cfg)
  │ cfg.JWKSURL empty? ──Yes──> return error, os.Exit(1) (fail fast)
  │ No
  │ fetch JWKS from cfg.JWKSURL
  │ fetch fails? ──Yes──> return error, os.Exit(1)
  │ No
  │ build jwksKeyStore, start background refresh goroutine
▼
Gateway serves requests; JWTAuthMiddleware calls GetKey(kid) per TD-001
│
[background, every cfg.JWKSRefreshInterval]
  refresh JWKS from cfg.JWKSURL
  success? ──Yes──> swap in updated key set
  │ No
  log refresh failure, keep serving last-known-good keys
```

---

# 7. Security

## Authentication

Signature verification continues to use `golang-jwt/jwt/v5` with the
explicit algorithm allowlist established in TD-001 — only the key-supply
mechanism changes, not the verification logic.

## Authorization

Not handled here — see TD-003.

## Data Protection

Public keys are non-secret by definition; `JWT_JWKS_URL` should be reachable
over TLS in production. TODO: confirm whether the gateway's HTTP client for
JWKS fetches needs custom TLS/cert-pinning config, not specified in the
plan.

## Rate Limiting

N/A for this component.

---

# 8. Performance

## Expected Load

Every request passes through `JWTAuthMiddleware`'s `GetKey` call
(TD-001); `jwksKeyStore.GetKey` remains an in-memory map lookup, so
per-request latency is unaffected by this change. Background refresh runs
on its own goroutine, off the request hot path.

## Database Impact

None.

## Caching Strategy

`jwkset.Storage` caches the fetched key set in memory; refreshed on
`cfg.JWKSRefreshInterval` (default 1h) via a background goroutine. Stale
keys remain usable across failed refreshes rather than the store going
empty.

---

# 9. Monitoring

## Metrics

- Reuses `jwt_validation_total{result}` / `jwt_validation_duration_seconds`
  from TD-001 (validation-path metrics are unaffected by this change)
- TODO: consider a `jwks_refresh_total{result="success|failure"}` metric —
  not specified in the plan, left to implementation

## Logging

- Background refresh failures logged via a `RefreshErrorHandler`-style
  callback (reason, `JWT_JWKS_URL`), non-fatal

## Alerts

- TODO: repeated JWKS refresh failures could indicate an auth-server
  outage — no specific alerting threshold defined in the plan

---

# 10. Risks

## Risk 1

`keyfunc`/`jwkset` API surface may not exactly match the reference shape
sketched in the plan — APIs changed significantly between `keyfunc` v2 and
v3.

Mitigation: confirm exact constructor/option names (e.g.
`keyfunc.NewDefaultCtx` vs. alternatives) against the vendored library's
current godoc during implementation, before finalizing `keystore.go`.

---

## Risk 2

A JWKS endpoint outage at gateway startup blocks the gateway from starting
at all (fail-fast, no fallback).

Mitigation: accepted tradeoff per the explicit "JWKS fully replaces static
keys, no fallback" project decision; rely on the auth server's own JWKS
endpoint availability/redundancy.

---

## Risk 3

Removing `SigningKeys`/`JWT_SIGNING_KEYS` is a breaking config change for
any existing deployment still setting that env var.

Mitigation: single-shot migration by design, no compatibility shim per the
"no fallback" decision; `TD-001`'s Open Questions section is updated to
document the new required env vars (`JWT_JWKS_URL`,
`JWT_JWKS_REFRESH_INTERVAL`).

---

# 11. Rollout Plan

## Deployment

1. `go get github.com/MicahParks/keyfunc/v3` (pulls in `jwkset`
   transitively); update `go.mod`/`go.sum`
2. Update `config/jwt.go`: remove `SigningKeys`/`JWT_SIGNING_KEYS`, add
   `JWKSURL`/`JWKSRefreshInterval`
3. Rewrite `internal/auth/keystore.go`: delete `staticKeyStore`, add
   `jwksKeyStore` backed by `jwkset.Storage`
4. Update `internal/auth/keystore_test.go` and `internal/auth/testing_test.go`
   with JWKS-serving `httptest` helpers; update
   `internal/auth/middleware_test.go` fixture construction
5. Update TD-001's key management Open Question section
6. Verify `go build ./...`, `go vet ./...`, and
   `go test ./internal/auth/... ./config/...` are all green

## Rollback

Revert the commit(s). `JWTAuthMiddleware`/`NewKeyStore` are not wired into
`cmd/api/main.go` or `internal/server` routing (per Non-Goals), so this
carries no live-traffic risk either way.

---

# 12. Open Questions

- Exact `keyfunc` v3 / `jwkset` constructor and option names (e.g.
  `keyfunc.NewDefaultCtx` vs. alternatives) — confirm against the vendored
  library's godoc during implementation.
- Whether to add a `jwks_refresh_total{result}` metric, or whether TD-001's
  existing `jwt_validation_total` is sufficient signal — not specified in
  the plan, left to the implementer.
- ~~Should the static-key startup coupling bug (LoadJWTConfig returning
  early, skipping JWT_SIGNING_KEYS parsing, whenever no issuing key is
  configured) be hotfixed independently before this migration lands?~~
  Resolved 2026-07-27: no separate hotfix. `SigningKeys` is removed
  entirely by this migration, which makes the bug moot; not worth a
  throwaway fix on a field this change deletes anyway.

---

# 13. ADR References

- ADR-001: CBAC with RBAC for Authentication and Authorization
- ADR-004: Use Go for API Gateway Implementation
