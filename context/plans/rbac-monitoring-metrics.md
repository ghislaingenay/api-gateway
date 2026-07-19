# Add RBAC monitoring metrics and startup logging

## Context

TD-002 (RBAC Data Model) §9 "Monitoring" specifies two metrics and a startup
log line that were not implemented as part of FEAT-002:

- `role_cache_load_duration_seconds` — how long `rbac.NewRoleCache` takes to
  load roles/permissions from PostgreSQL at startup.
- `roles_endpoint_requests_total` — request counter for `GET /roles` (and, by
  extension, `GET /permissions`).
- A startup log line confirming roles/permissions were loaded, with counts of
  each.

None of this exists today because the codebase has no metrics library in
`go.mod` yet — instrumenting just these two RBAC metrics would mean either
introducing a metrics dependency (e.g. `prometheus/client_golang`) for a
single feature, or hand-rolling counters with no way to expose/scrape them.
That's a bigger decision than this feature's scope, so it's deferred here
rather than bolted on ad hoc.

## Why this shape

- `internal/rbac/repository.go`'s `NewRoleCache(ctx context.Context, db
  database.Service) (RoleCache, error)` is the natural place to time the
  load and emit the startup log — it already knows role/permission counts
  after `loadRoles`/`loadPermissions` return.
- `internal/server/routes.go`'s `GET /roles` / `GET /permissions`
  registrations (via `s.requirePermission`) are the natural place to wrap
  request counting, once a metrics client exists to record against.
- Whatever metrics library gets chosen should be decided project-wide, not
  per-feature — check if a later feature (rate limiting, TD-005, is the
  other place in the project overview that mentions metrics/monitoring) ends
  up picking one first, and reuse that choice here instead of introducing a
  second library.

## Changes

### 1. Startup log (low effort, no new dependency)

In `internal/rbac/repository.go`, after `NewRoleCache` successfully loads
roles and permissions, add a single `log.Printf` (or whatever logging
convention the project settles on) reporting counts, e.g.:

```go
log.Printf("rbac: loaded %d roles, %d permissions", len(roles), len(permissions))
```

This has no new dependencies and can land independently of the metrics work
below.

### 2. Metrics library selection

- Confirm whether another in-flight/planned feature (e.g. rate limiting)
  already needs to pick a metrics library — if so, adopt that choice here
  rather than deciding independently.
- If nothing else has decided yet, default to `prometheus/client_golang`
  (the de facto standard for Go services) with a `/metrics` endpoint
  exposed alongside `/health` in `internal/server/routes.go`.

### 3. `role_cache_load_duration_seconds`

- A `prometheus.Histogram` (or `Summary`), registered once at startup.
- Recorded around the `loadRoles`/`loadPermissions` calls inside
  `NewRoleCache`.

### 4. `roles_endpoint_requests_total`

- A `prometheus.CounterVec` labeled by endpoint (`/roles`, `/permissions`)
  and status code.
- Wrap `rbac.RolesHandler`/`rbac.PermissionsHandler` (or add a small
  metrics-recording middleware alongside `s.requirePermission` in
  `internal/server/routes.go`) to increment it per request.

## Out of scope

- Instrumenting any endpoint outside `/roles` and `/permissions` — this plan
  only covers what TD-002 specified for RBAC.
- Picking the metrics backend/dashboard (Grafana, Datadog, etc.) that
  scrapes/visualizes these — only the in-process metric emission.

## Verification

- `go build ./...` and `go vet ./...` pass.
- `go test ./internal/rbac/... ./internal/server/...` passes, including a
  test asserting the startup log line appears (or the histogram/counter
  values change) after `NewRoleCache`/a request.
- Manually curl `/metrics` (once added) and confirm both metrics appear with
  sane values after a few `/roles` and `/permissions` requests.
