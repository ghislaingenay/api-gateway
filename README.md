# API Gateway

A production-grade, multi-tenant API gateway written in Go: Keycloak-backed
JWT authentication with server-side tenant/role resolution
(RBAC/CBAC), distributed rate limiting, response caching, request
validation, resilient proxying (retry + timeout), and structured
observability, all in front of your downstream services.

## Architecture

```
                                   ┌──────────────────────────────┐
                                   │        API Gateway           │
Client ──HTTP──▶ CorrelationID ──▶│ CORS ─▶ ServeMux              │
                                   │                               │
                                   │  /health, /ready, /docs       │──▶ (no auth, no rate limit, no cache)
                                   │  /roles, /permissions         │──▶ JWT auth + permission check
                                   │  /auth/me, /onboarding        │──▶ JWT auth (no tenant required)
                                   │                               │
                                   │  /api/*  (proxied routes)     │
                                   │    JWT auth (Keycloak JWKS)   │
                                   │      → tenant/role resolution │
                                   │        → request validation   │
                                   │          → rate limiting      │
                                   │            → response cache   │
                                   │              → resilient proxy│──▶ downstream service (retry + deadline)
                                   └───────┬───────┬───────────────┘
                                           │       │
                                     ┌─────▼──┐ ┌──▼───┐
                                     │Postgres│ │ Redis│
                                     └────────┘ └──────┘
```

Login, token issuance, and refresh all happen directly against **Keycloak**
(its own container + database in `docker-compose.yml`) — the gateway never
sees a password. Tenant identity is a client-supplied `X-Tenant-ID` header,
re-verified against a `tenant_users` table on every request rather than
trusted from the JWT (see [TD-012](context/technical-designs/TD-012-keycloak-identity-provider.md#7-security)).

Route → upstream mapping, auth/permission requirements, cache TTLs, retry
policy, and validation rules are all declared statically in
[`config/routes.json`](config/routes.json) and loaded into the gateway's
route table at startup — no code change is needed to add a new proxied
route.

## API Documentation

The gateway serves interactive Swagger UI at **`/docs`** (backed by the
OpenAPI 3.0 spec at `/docs/openapi.yaml`, source:
[`internal/apidocs/openapi.yaml`](internal/apidocs/openapi.yaml)), covering
every gateway-managed endpoint: health/readiness, auth, RBAC catalog, and the
proxied `/api/*` routes.

## Local Development

Everything needed to run the gateway locally is in `docker-compose.yml` —
no external paid services required.

```bash
cp .env.example .env
docker compose up --build
```

`.env.example` ships with fake local-only Postgres/Redis credentials and a
dedicated JWT signing keypair generated just for local dev, so the stack
works out of the box.

This starts, in dependency order:

1. **postgres** (`localhost:5433`) and **redis** (`localhost:6380`) — health-checked before anything else starts.
2. **keycloak-db** and **keycloak** (`localhost:8090`) — the identity provider, with its realm (`api-gateway`), clients (`client-frontend`, `api-gateway-backend`), and seeded demo users auto-imported from [`keycloak/realm-export.json`](keycloak/realm-export.json) on first boot.
3. **migrate** — a one-shot job (`cmd/migrate`) that applies all pending database migrations, then exits. The gateway only starts once this completes successfully.
4. **gateway** (`localhost:8080`) — the API gateway itself, verifying tokens against Keycloak's realm JWKS endpoint.
5. **orders-service** (`localhost:8081`) — a minimal mock downstream service (`cmd/mockorders`) that `config/routes.json` proxies `/api/orders/*` to, so you can see the gateway's full request flow (auth → tenant resolution → validation → rate limit → cache → resilient proxy) end-to-end.

Then seed a tenant and the gateway-side rows for the two Keycloak demo users (`admin@seed.test` / `viewer@seed.test`, password `password123`) against the compose Postgres:

```bash
APP_ENV=development DB_HOST=localhost DB_PORT=5433 \
DB_DATABASE=gateway DB_USER=gateway DB_PASSWORD=gateway \
DB_SSL_MODE=disable DB_SCHEMA=public \
go run ./cmd/seed
```

(`make dbflush` does a full drop/re-migrate/reseed in one step if you need
to reset local state.)

Open [http://localhost:8080/docs](http://localhost:8080/docs) for Swagger
UI. Login now happens against Keycloak directly, not the gateway — sign in
at [http://localhost:8090/realms/api-gateway/account](http://localhost:8090/realms/api-gateway/account)
with a demo user's credentials to obtain a JWT (or drive `client-frontend`'s
PKCE flow from a real frontend), then call the gateway with it:

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/auth/me

curl -H "Authorization: Bearer $TOKEN" -H "X-Tenant-ID: <tenant-id-from-auth-me>" \
  http://localhost:8080/api/orders/$(python3 -c 'import uuid;print(uuid.uuid4())')
```

Shut the stack down with `docker compose down` (add `-v` to also drop the
Postgres/Redis volumes).

### Ports

Postgres and Redis are mapped to non-default host ports (`5433`, `6380`)
to avoid clashing with any Postgres/Redis instance you may already be
running locally.

### Images

- `Dockerfile` builds the production gateway image (`cmd/api` only).
- `Dockerfile.dev` builds a local-development-only image bundling
  `cmd/migrate`, `cmd/mockorders`, `cmd/seed`, and `cmd/dbflush` — never
  shipped to production. Compose builds it once and reuses it across the
  `migrate` and `orders-service` services via `command:` overrides.

## Load Testing

A [k6](https://k6.io/docs/get-started/installation/) script at
[`loadtest/gateway-load-test.js`](loadtest/gateway-load-test.js) load-tests
the running gateway (auth, cache, rate limiting, and the proxied
`/api/orders` routes) with a ramping profile up to 1000 virtual users.

**Run it manually, locally:**

```bash
docker compose up --build -d
go run ./cmd/seed   # if you haven't already seeded a tenant/user
k6 run loadtest/gateway-load-test.js
```

Auth is against Keycloak directly via a dedicated `client-loadtest` client
(direct-access-grants only, no PKCE needed for scripting). Override the
target or credentials with `-e`, e.g.
`k6 run -e BASE_URL=... -e KEYCLOAK_URL=... -e LOGIN_EMAIL=... loadtest/gateway-load-test.js`.

**Run it in GitHub Actions:** add the `load-test` label to a pull request.
This triggers [`.github/workflows/load-test.yml`](.github/workflows/load-test.yml),
which spins up the full compose stack, seeds it, and runs the same script
against it. While the label is present, it re-triggers automatically on
every new push to the PR; removing the label cancels an in-progress run.
If the `load-test` label doesn't exist yet in this repo, create it first
under the repo's Issues/PRs → Labels settings.

## MakeFile

Run build make command with tests
```bash
make all
```

Build the application
```bash
make build
```

Run the application
```bash
make run
```

Create DB container
```bash
make docker-run
```

Shutdown DB Container
```bash
make docker-down
```

DB Integrations Test:
```bash
make itest
```

Live reload the application:
```bash
make watch
```

Run the test suite:
```bash
make test
```

Clean up binary from the last build:
```bash
make clean
```

## Design Decisions

Each feature's technical design doc records its architecture, data model,
and key trade-offs:

- [TD-000: Core Identity Data Model](context/technical-designs/TD-000-core-identity-data-model.md)
- [TD-001: JWT Authentication (CBAC)](context/technical-designs/TD-001-jwt-authentication.md)
- [TD-002: RBAC Data Model](context/technical-designs/TD-002-rbac-data-model.md)
- [TD-003: Authorization Enforcement](context/technical-designs/TD-003-authorization-enforcement.md)
- [TD-004: Multi-Tenant Isolation & Routing](context/technical-designs/TD-004-multi-tenant-routing.md)
- [TD-005: Distributed Rate Limiting](context/technical-designs/TD-005-distributed-rate-limiting.md)
- [TD-006: Response Caching](context/technical-designs/TD-006-response-caching.md)
- [TD-007: Request Validation](context/technical-designs/TD-007-request-validation.md)
- [TD-008: Resilience (Retry & Timeout)](context/technical-designs/TD-008-resilience-retry-timeout.md)
- [TD-009: Observability & Health Checks](context/technical-designs/TD-009-observability-health-checks.md)
- [TD-010: API Documentation & Dev Environment](context/technical-designs/TD-010-api-docs-dev-environment.md)
- [TD-011: JWKS Key Rotation](context/technical-designs/TD-011-jwks-key-rotation.md)
- [TD-012: Keycloak as Identity Provider](context/technical-designs/TD-012-keycloak-identity-provider.md)

The full feature index is at [context/features/README.md](context/features/README.md).

## Roadmap

Planned next steps:

- **Bot detection** — identify and rate-limit/block automated/bot traffic hitting the gateway, beyond today's per-tenant rate limiting.
- **Hot reload on file change** — reload the running gateway (config/routes, and ideally code) when a watched file changes, without a manual restart.
- **Inviting members into an existing tenant** — FEAT-012's onboarding only covers a tenant's first user (its owner); adding further members is left to a future feature.
- **mTLS for service-to-service authentication** — Keycloak's realm signing keypair is the JWT trust root; certificate-based service auth for downstream calls is separate and still open.
- **Cloud deployment** — a production deployment target (container registry, orchestration, managed Postgres/Redis, secrets management) beyond the current local-only `docker-compose.yml`.

## Contributing

1. Fork the repo and create a branch off `master` for your change.
2. Follow the existing patterns: business logic in `internal/`, shared config loading in `config/`, one `cmd/*` binary per entrypoint.
3. Add or update tests alongside your change (`make test`, or `make itest` for the database integration suite) and make sure `go vet ./...` is clean.
4. If your change affects the local dev stack, verify it end-to-end against `docker-compose.yml` (see [Local Development](#local-development)) before opening a PR.
5. For anything beyond a small fix, add or update the relevant technical design doc under `context/technical-designs/` and feature entry under `context/features/` — see [Design Decisions](#design-decisions) for the existing index.
6. Open a PR against `master` with a clear description of the change and why it's needed.
