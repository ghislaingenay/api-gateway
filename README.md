# API Gateway

A production-grade, multi-tenant API gateway written in Go: JWT authentication
with role/permission-based access control (RBAC/CBAC), distributed rate
limiting, response caching, request validation, resilient proxying (retry +
timeout), and structured observability, all in front of your downstream
services.

## Architecture

```
                                   ┌──────────────────────────────┐
                                   │        API Gateway           │
Client ──HTTP──▶ CorrelationID ──▶│ CORS ─▶ ServeMux              │
                                   │                               │
                                   │  /health, /ready, /docs       │──▶ (no auth, no rate limit, no cache)
                                   │  /roles, /permissions         │──▶ JWT auth + permission check
                                   │  /auth/login, /auth/refresh   │──▶ public
                                   │  /auth/logout, /auth/me       │──▶ JWT auth
                                   │                               │
                                   │  /api/*  (proxied routes)     │
                                   │    JWT auth                   │
                                   │      → request validation     │
                                   │        → rate limiting        │
                                   │          → response cache     │
                                   │            → resilient proxy  │──▶ downstream service (retry + deadline)
                                   └───────┬───────┬───────────────┘
                                           │       │
                                     ┌─────▼──┐ ┌──▼───┐
                                     │Postgres│ │ Redis│
                                     └────────┘ └──────┘
```

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
docker compose up --build
```

This starts, in dependency order:

1. **postgres** (`localhost:5433`) and **redis** (`localhost:6380`) — health-checked before anything else starts.
2. **migrate** — a one-shot job (`cmd/migrate`) that applies all pending database migrations, then exits. The gateway only starts once this completes successfully.
3. **gateway** (`localhost:8080`) — the API gateway itself.
4. **orders-service** (`localhost:8081`) and **inventory-service** (`localhost:8082`) — minimal mock downstream services (`cmd/mockorders`, `cmd/mockinventory`) that `config/routes.json` proxies `/api/orders/*` and `/api/inventory/*` to, so you can see the gateway's full request flow (auth → validation → rate limit → cache → resilient proxy) end-to-end.

Then seed a tenant and two test users (`admin@seed.test` / `viewer@seed.test`, password `password123`) against the compose Postgres:

```bash
APP_ENV=development DB_HOST=localhost DB_PORT=5433 \
DB_DATABASE=gateway DB_USER=gateway DB_PASSWORD=gateway \
DB_SSL_MODE=disable DB_SCHEMA=public \
go run ./cmd/seed
```

Open [http://localhost:8080/docs](http://localhost:8080/docs) for Swagger
UI, or try the full flow from the command line:

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@seed.test","password":"password123","tenant_slug":"seed-tenant"}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["access_token"])')

curl -H "Authorization: Bearer $TOKEN" \
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
  `cmd/migrate`, `cmd/mockorders`, and `cmd/mockinventory` — never shipped
  to production. Compose builds it once and reuses it across the
  `migrate`, `orders-service`, and `inventory-service` services via
  `command:` overrides.

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

The full feature index is at [context/features/README.md](context/features/README.md).
