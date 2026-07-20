package tenant

import (
	"context"
	"errors"
	"fmt"
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

const (
	statusActive   = "1"
	statusInactive = "0"
)

// StatusChecker reports whether a tenant is currently active (not disabled,
// not soft-deleted).
type StatusChecker interface {
	IsActive(ctx context.Context, tenantID uuid.UUID) (bool, error)
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

// NewStatusCache returns a StatusChecker backed by Redis, falling back to
// repo on a cache miss and populating the cache with the result. A tenant
// that no longer exists is treated as inactive rather than as an error, so
// gateway callers fail closed (403) instead of erroring (500).
func NewStatusCache(repo Repository, redisClient *redis.Client, ttl time.Duration) StatusChecker {
	return &redisStatusCache{repo: repo, redis: redisClient, ttl: ttl}
}

// IsActive implements StatusChecker.
func (c *redisStatusCache) IsActive(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	key := statusCacheKeyPrefix + tenantID.String()

	cached, err := c.redis.Get(ctx, key).Result()
	if err == nil {
		return cached == statusActive, nil
	}
	if !errors.Is(err, redis.Nil) {
		return false, fmt.Errorf("read tenant status cache: %w", err)
	}

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
