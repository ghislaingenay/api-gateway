# Current Feature

FEAT-007: Request Validation

## File

[FEAT-007-request-validation.md](features/FEAT-007-request-validation.md)

## Goals

- [ ] FR-1: Gateway validates JSON request bodies against a per-route schema (struct validation tags consistent with the project's data models)
- [ ] FR-2: Gateway validates required path/query parameters per route configuration
- [ ] FR-3: Validation errors are returned in a consistent JSON error format across all routes

## Notes

Middleware validates body/params against a per-route schema resolved from the static route table, runs after auth and before rate limiting (auth -> validation -> ratelimit -> cache -> gateway), and fails closed with 400 on any validation failure. Schemas are data-driven (JSON config, like FEAT-006's CacheTTL) rather than tied to compile-time Go structs, since routed bodies belong to external upstream services this repo doesn't own. Branch: feat-007/request-validation.
