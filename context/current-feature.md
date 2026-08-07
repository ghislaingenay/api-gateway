# Current Feature

FEAT-012: Keycloak as Identity Provider

## File

[FEAT-012 - Keycloak as Identity Provider](features/FEAT-012-keycloak-identity-provider.md)

## Goals

- [x] FR-1: Gateway verifies Keycloak-issued JWTs via realm JWKS, rejects wrong issuer/disallowed algorithm
- [x] FR-2: Gateway accepts `X-Tenant-ID` but re-verifies membership on every request
- [x] FR-3: Gateway JIT-provisions a local `users` row on first sight of a new Keycloak `sub`
- [x] FR-4: Authenticated user with zero tenant memberships can create a tenant via `POST /onboarding`
- [x] FR-5: Role/permission resolution comes from `tenant_users` → `roles` → `role_permissions`, cached, never JWT claims
- [x] FR-6: `GET /auth/me` returns identity + tenant memberships or tenant-scoped role/profile
- [x] FR-7: `POST /auth/login`, `/auth/refresh`, `/auth/logout` removed from the gateway (404, not disabled)

## Notes

Replaces the gateway's self-hosted login/refresh/logout and credential
storage with Keycloak (docker-compose, realm `api-gateway`, client
`client-frontend`) as external OIDC IdP. Gateway becomes tenant/RBAC
authority: tenant context resolved server-side per request from
`X-Tenant-ID` against `tenant_users`, cached (in-process + Redis,
`IDENTITY_CACHE_TTL=15m`). New `internal/identity` and
`internal/onboarding` packages. See TD-012 for full design.
