# FEAT-011: JWKS Key Rotation

Status: Done

Owner: Ghislain Genay
Created: 2026-07-27
Last Updated: 2026-07-27

Technical Design: [TD-011 - JWKS Key Rotation](../technical-designs/TD-011-jwks-key-rotation.md)

---

# 1. Overview

## Summary

Replace the gateway's static, env-var-driven JWT signing-key store
(`JWT_SIGNING_KEYS`, restart-to-rotate) with a JWKS-backed store that fetches
and caches public keys from an auth/identity server's `/.well-known/jwks.json`
endpoint, refreshing automatically in the background. New keys become active
the moment the JWKS endpoint is updated — no gateway restart required.

## Problem

Rotating a JWT signing key today means updating the `JWT_SIGNING_KEYS` env
var and restarting every gateway instance, which is manual, error-prone, and
was flagged in TD-001 as a deliberate MVP simplification that already
anticipated JWKS as the natural next step. The current static implementation
also has a coupling bug where a gateway instance without an issuing key
configured (`JWT_SIGNING_KID`/`JWT_SIGNING_PRIVATE_KEY`) fails to start even
if verification keys (`JWT_SIGNING_KEYS`) are fully populated — this
migration removes `SigningKeys` entirely, which makes that bug moot rather
than requiring a separate hotfix.

## Goals

- Fetch signing public keys from a configured JWKS URL instead of a static
  env var
- Refresh keys automatically in the background on a configurable interval,
  with no gateway restart needed to pick up rotation
- Preserve the existing `KeyStore` interface so `middleware.go`, `claims.go`,
  `context.go`, and `errors.go` need zero changes
- Fail fast at startup if the JWKS URL is unset or unreachable

## Non-Goals

- Any static-key fallback or dev-mode path — JWKS fully replaces static keys
- Building/publishing the JWKS endpoint itself (that's the auth/identity
  server's responsibility, not this gateway)
- Wiring `JWTAuthMiddleware`/`NewKeyStore` into `cmd/api/main.go` or
  `internal/server` routing — matches the current unwired state, separate
  follow-up
- Changes to token-issuing key configuration (`JWT_SIGNING_KID` /
  `JWT_SIGNING_PRIVATE_KEY` stay as-is)

---

# 2. Users

## Primary Users

- Platform/gateway operators, who no longer need to update env vars and
  restart every instance to rotate a key
- Downstream services/API clients relying on uninterrupted token validation
  during rotation

## Stakeholders

- Engineering (gateway implementation)
- Security (key rotation is a security-hygiene operation)

---

# 3. User Stories

### Story 1

As a platform operator

I want new signing keys to become active automatically when the auth server
publishes them to JWKS

So that I don't have to update env vars and restart every gateway instance
to rotate a key

### Story 2

As a platform operator

I want the gateway to keep validating tokens with the last-known-good keys
if the JWKS endpoint is briefly unreachable

So that a transient network blip doesn't cause a token-validation outage

---

# 4. Product Requirements

## Functional Requirements

### FR-1

The gateway must fetch signing public keys from a configured JWKS URL
(`JWT_JWKS_URL`) instead of the static `JWT_SIGNING_KEYS` env var.

#### Acceptance Criteria

- [x] `JWT_SIGNING_KEYS` env var and `SigningKeys` config field are removed
- [x] `NewKeyStore` builds its key set from `JWT_JWKS_URL`
- [x] Startup fails with a clear error if `JWT_JWKS_URL` is unset or the
      initial fetch fails

---

### FR-2

The key store must refresh keys in the background on a configurable
interval, without requiring a gateway restart.

#### Acceptance Criteria

- [x] `JWT_JWKS_REFRESH_INTERVAL` configures refresh cadence, defaulting to
      a sane value (e.g. 1h) when unset or invalid
- [x] A background refresh failure is logged but does not crash the
      process; last-known-good keys remain usable until the next successful
      refresh
- [x] A key added to the JWKS endpoint after startup becomes resolvable via
      `GetKey` after the store's next refresh, with no restart

---

### FR-3

Key lookup by `kid` must continue to satisfy the existing `KeyStore`
interface and error semantics so downstream code is unaffected.

#### Acceptance Criteria

- [x] `GetKey(kid)` returns the `*rsa.PublicKey` for a known `kid`
- [x] `GetKey(kid)` for an unknown `kid` returns `ErrUnknownKey` (unchanged
      401 behavior in `middleware.go`)
- [x] Non-RSA keys served by the JWKS endpoint are rejected rather than
      silently accepted

---

## Business Rules

- JWKS fully replaces static keys — no fallback mode (explicit project
  decision, not a phased rollout)
- Issuing-key configuration (`JWT_SIGNING_KID`/`JWT_SIGNING_PRIVATE_KEY`)
  remains independent of and unaffected by this change

---

## Permissions

N/A — internal key-management change, no new user-facing authorization
surface.

---

## User Flow

1. Auth/identity server publishes public keys at its JWKS endpoint (out of
   scope, external system)
2. Gateway process starts; `NewKeyStore` fetches the current key set from
   `JWT_JWKS_URL`, failing startup if unreachable
3. Gateway serves requests, resolving `kid` → public key from the cached
   JWKS storage
4. Auth server rotates a key: publishes a new `kid` and/or retires an old one
5. Gateway's background refresh (`JWT_JWKS_REFRESH_INTERVAL`) picks up the
   new key set automatically
6. Tokens signed with the new `kid` validate; tokens with a retired `kid`
   are rejected once it drops from the served JWKS document

---

# 5. Edge Cases

- JWKS endpoint unreachable at startup → `NewKeyStore` returns an error,
  gateway fails to start (fail fast, no fallback)
- JWKS endpoint temporarily unreachable during a background refresh → stale
  keys remain usable, refresh error logged, process keeps running
- JWKS response contains a non-RSA key (e.g. EC) → rejected for that `kid`
- `kid` absent from the current JWKS document → `ErrUnknownKey`, 401
- Known static-store startup bug (see Problem) is subsumed by this
  migration since `SigningKeys` is removed entirely — no separate hotfix
  planned

---

# 6. Dependencies

## Internal

- [[FEAT-001]] JWT Authentication (CBAC) — this feature replaces FEAT-001's
  static key-store implementation without changing its middleware/claims
  contract (`KeyStore` interface stays the same)

## External

- `github.com/MicahParks/keyfunc/v3`
- `github.com/MicahParks/jwkset` (pulled in transitively)

## Prerequisites

- FEAT-001 implemented (`KeyStore` interface, `JWTAuthMiddleware`,
  `CustomClaims` already exist and are reused unchanged)

---

# 7. Success Criteria

## Business Metrics

- Zero gateway restarts required for key rotation going forward (down from
  one per rotation today)

## Technical Metrics

- `go build ./...` and `go vet ./...` pass
- `go test ./internal/auth/... ./config/...` passes, including new JWKS
  keystore tests (known kid, unknown kid, unreachable endpoint, rotation)

---

# 8. Related Documents

- Technical Design: TD-011
- FEAT-001: JWT Authentication (CBAC)
- TD-001: JWT Authentication (CBAC)
- Plan: `context/plans/jwks-key-rotation.md`
