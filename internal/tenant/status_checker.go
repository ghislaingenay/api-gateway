package tenant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// StatusCacheTTL bounds how long a cached tenant active-status entry may be
// served before falling back to the database. It trades a small staleness
// window (a just-deactivated tenant can still be routed for up to this long)
// for keeping the active-status check off the request's DB path.
const StatusCacheTTL = 30 * time.Second

const statusCacheKeyPrefix = "tenant:status:"
const limitsCacheKeyPrefix = "tenant:ratelimits:"

const (
	statusActive   = "1"
	statusInactive = "0"
)

// StatusChecker reports whether a tenant is currently active (not disabled,
// not soft-deleted).
type StatusChecker interface {
	IsActive(ctx context.Context, tenantID uuid.UUID) (bool, error)
}

// RateLimits holds a tenant's configured per-minute and per-hour request
// limits, as stored on the tenants table.
type RateLimits struct {
	PerMinute int
	PerHour   int
}

// RateLimitProvider resolves a tenant's configured rate limits.
type RateLimitProvider interface {
	RateLimits(ctx context.Context, tenantID uuid.UUID) (RateLimits, error)
}

// statusCacheStore is the subset of *redis.Client the status cache needs,
// sized to its two calls so tests can substitute a fake without a live
// Redis instance.
type statusCacheStore interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd
}

type redisStatusCache struct {
	repo  Repository
	redis statusCacheStore
	ttl   time.Duration
}

// NewStatusCache returns a Redis-backed cache satisfying both StatusChecker
// and RateLimitProvider, falling back to repo on a cache miss and
// populating the cache with the result. A tenant that no longer exists is
// treated as inactive rather than as an error, so gateway callers fail
// closed (403) instead of erroring (500).
func NewStatusCache(repo Repository, redisClient *redis.Client, ttl time.Duration) *redisStatusCache {
	return &redisStatusCache{repo: repo, redis: redisClient, ttl: ttl}
}

// IsActive implements StatusChecker.
func (c *redisStatusCache) IsActive(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	key := statusCacheKeyPrefix + tenantID.String()

	if cached, err := c.redis.Get(ctx, key).Result(); err == nil {
		return cached == statusActive, nil
	}
	// Cache miss or Redis error: best-effort cache, fall back to the
	// database rather than failing the request when Redis is unavailable.

	t, err := c.repo.GetByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, ErrTenantNotFound) {
			_ = c.redis.Set(ctx, key, statusInactive, c.ttl).Err()
			return false, nil
		}
		return false, fmt.Errorf("load tenant for status check: %w", err)
	}

	active := t.IsActive && t.DeletedAt == nil
	value := statusInactive
	if active {
		value = statusActive
	}
	_ = c.redis.Set(ctx, key, value, c.ttl).Err()

	return active, nil
}

// RateLimits implements RateLimitProvider.
func (c *redisStatusCache) RateLimits(ctx context.Context, tenantID uuid.UUID) (RateLimits, error) {
	key := limitsCacheKeyPrefix + tenantID.String()

	if cached, err := c.redis.Get(ctx, key).Result(); err == nil {
		var limits RateLimits
		if jsonErr := json.Unmarshal([]byte(cached), &limits); jsonErr == nil {
			return limits, nil
		}
		// Corrupt cache entry: fall through to the database.
	}
	// Cache miss or Redis error: best-effort cache, fall back to the
	// database rather than failing the request when Redis is unavailable.

	t, err := c.repo.GetByID(ctx, tenantID)
	if err != nil {
		// Don't propagate a Postgres failure as a rate-limit error: doing so
		// would make the caller fail open (skip rate limiting entirely) for
		// a DB outage, not just a Redis outage. Log it and fall back to a
		// zero-value RateLimits instead, which callers resolve against their
		// own configured defaults, so rate limiting still applies.
		log.Printf("tenant: failed to load rate limits for tenant %s, falling back to defaults: %v", tenantID, err)
		return RateLimits{}, nil
	}

	limits := RateLimits{PerMinute: t.RateLimitPerMinute, PerHour: t.RateLimitPerHour}
	if encoded, jsonErr := json.Marshal(limits); jsonErr == nil {
		_ = c.redis.Set(ctx, key, encoded, c.ttl).Err()
	}

	return limits, nil
}
