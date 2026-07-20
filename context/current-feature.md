# Current Feature

FEAT-006: Response Caching

## File

[FEAT-006-response-caching.md](features/FEAT-006-response-caching.md)

## Goals

- [x] FR-1: Gateway checks Redis for a cached response before forwarding a GET request downstream, using key format `cache:{tenant_id}:{method}:{path}:{query_hash}`
- [x] FR-2: Cached responses respect a per-route configurable TTL
- [x] FR-3: Only successful (2xx) GET responses are cached; error responses and non-GET methods are never cached

## Notes

Redis-backed response cache for GET requests, tenant-scoped by construction (key always derives tenant_id from validated JWT claims). Fails open to a downstream call on Redis errors/miss. Sits between ratelimit.RateLimitMiddleware and gateway.NewHandler in the /api/ chain. Default TTL: 60s (user decision). Cache-Control: no-store from downstream is ignored for MVP (user decision). Branch: feat-006/response-caching (already checked out; FEAT-005 dependency merged from master).
