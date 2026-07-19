# Switch JWT key management to JWKS (automatic key rotation)

## Context

The gateway's JWT auth currently loads RSA public keys from a static env var
(`JWT_SIGNING_KEYS`, a `kid=base64pem` list) at process startup
(`config/jwt.go` → `config.LoadJWTConfig`), fed into an in-memory
`staticKeyStore` (`internal/auth/keystore.go`). Rotating a key today means
updating the env var and restarting every gateway instance — TD-001 flagged
this as a deliberate MVP simplification ("No KMS/JWKS integration for MVP")
but already anticipated JWKS as the natural next step (section 8: "Public
keys/JWKS cached in memory ... on rotation events").

We're replacing the static store with a JWKS-backed store: the auth server
publishes its public keys at a `/.well-known/jwks.json`-style URL, and the
gateway fetches/caches/refreshes them automatically, keyed by `kid`. This
removes manual key-rotation ops entirely — new keys become active the moment
the JWKS endpoint is updated, with no gateway restart.

Per user decisions: **JWKS fully replaces static keys** (no fallback mode),
implemented with **`github.com/MicahParks/keyfunc/v3` + its underlying
`github.com/MicahParks/jwkset`** (the library pairing designed for
`golang-jwt/jwt/v5`, which is already the JWT library in `go.mod`). This
change stays **scoped to `internal/auth` + `config`** — wiring
`JWTAuthMiddleware` into the actual server/router is out of scope (it isn't
wired in today either; that's a separate follow-up).

## Why this shape

- `internal/auth/keystore.go` already defines `KeyStore` as a narrow
  interface — `GetKey(kid string) (*rsa.PublicKey, error)` — consumed only
  by `middleware.go`'s keyfunc closure. That seam means **`middleware.go`,
  `claims.go`, `context.go`, and `errors.go` need zero changes**; only
  `keystore.go` and `config/jwt.go` change.
- `internal/auth/testing_test.go` already has RSA keypair + PEM-encoding
  test helpers reusable for building an `httptest.Server` that serves a JWKS
  JSON document.

## Changes

### 1. `config/jwt.go`

- Remove `SigningKeys map[string]string` and all `JWT_SIGNING_KEYS` parsing.
- Add:
  - `JWKSURL string` — from required env var `JWT_JWKS_URL`.
  - `JWKSRefreshInterval time.Duration` — from optional `JWT_JWKS_REFRESH_INTERVAL`
    (parsed with `time.ParseDuration`), defaulting to a sane value (e.g. 1h)
    if unset/invalid — background refresh cadence for the JWKS client.
- Keep `AllowedAlgorithms` / `JWT_ALLOWED_ALGORITHMS` parsing unchanged.
- `LoadJWTConfig` no longer needs to return a usable config when the JWKS URL
  is empty — `NewKeyStore` (see below) should error out clearly instead of
  silently building an empty key map, since there's no more fallback.

### 2. `internal/auth/keystore.go`

- Delete `staticKeyStore` and its PEM-decoding logic.
- Add a `jwksKeyStore` that wraps a `jwkset.Storage` (obtained via
  `keyfunc.NewDefaultCtx`/`jwkset`'s HTTP-client constructor pointed at
  `cfg.JWKSURL`, configured with the refresh interval from config). Exact
  constructor/option names should be confirmed against the vendored
  library's current godoc during implementation (APIs across `keyfunc`
  v2→v3 changed significantly) — the intended shape is:
  - Build storage/client once in `NewKeyStore`, with a background refresh
    goroutine tied to `cfg.JWKSRefreshInterval` and an error handler that
    logs refresh failures (mirroring the `RefreshErrorHandler` pattern from
    the reference snippet) without crashing the process — stale keys stay
    usable until the next successful refresh.
  - `GetKey(kid string) (*rsa.PublicKey, error)` looks up the key via the
    storage's read-by-kid method, type-asserts the returned key to
    `*rsa.PublicKey` (reject non-RSA keys), and maps "key not found" to the
    existing `ErrUnknownKey` sentinel so `middleware.go`'s error handling
    (→ 401) keeps working unchanged.
- `NewKeyStore(cfg *config.JWTConfig) (KeyStore, error)` signature stays the
  same so callers don't change; it now always builds a `jwksKeyStore` and
  returns an error if `cfg.JWKSURL` is empty or the initial fetch fails
  (fail fast at startup rather than serving traffic with no keys).

### 3. Dependencies

- `go get github.com/MicahParks/keyfunc/v3` (pulls in
  `github.com/MicahParks/jwkset` transitively) — update `go.mod`/`go.sum`.

### 4. Tests (`internal/auth/keystore_test.go`, `testing_test.go`)

- Replace static-key-store test cases with JWKS-store tests using
  `httptest.NewServer` serving a hand-built JWKS JSON body (a small new
  helper to encode an `*rsa.PublicKey` as a JWK — base64url `n`/`e` — reusing
  the existing `generateRSAKeyPair` helper for key generation).
- Cover: known `kid` resolves correctly; unknown `kid` → `ErrUnknownKey`;
  JWKS endpoint unreachable at startup → `NewKeyStore` returns an error;
  (if feasible without excessive test complexity) a rotation scenario —
  serve one key set, then swap the handler's response and confirm a new
  `kid` becomes resolvable after the store's next refresh.
- `middleware_test.go` should need no logic changes, only updating however
  it currently constructs a `KeyStore` for its test fixtures (via the new
  JWKS test helper instead of the static one).

### 5. Docs (light touch)

- Update `context/technical-designs/TD-001-jwt-authentication.md`'s key
  management section/Open Questions to record the JWKS decision (superseding
  "No KMS/JWKS integration for MVP") and note the new env vars
  (`JWT_JWKS_URL`, `JWT_JWKS_REFRESH_INTERVAL` replacing `JWT_SIGNING_KEYS`).

## Out of scope

- Wiring `JWTAuthMiddleware`/`NewKeyStore` into `cmd/api/main.go` or
  `internal/server` routing — matches current unwired state.
- Building the JWKS-publishing side (that lives on the auth/identity server,
  not this gateway).
- Any static-key fallback/dev-mode path.

## Verification

- `go build ./...` and `go vet ./...` pass.
- `go test ./internal/auth/... ./config/...` passes, including the new JWKS
  keystore tests (known kid, unknown kid, unreachable endpoint, rotation).
- Manually sanity-check via a short `go run` snippet or test: point
  `JWT_JWKS_URL` at an `httptest.Server`, call `NewKeyStore`, then
  `GetKey(kid)` for a key present in the served JWKS and confirm it matches
  the original RSA public key.
