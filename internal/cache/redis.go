package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const keyPrefix = "cache:"

// NewRedisClient initializes and tests the Redis connection.
func NewRedisClient(url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return client, nil
}

// BuildKey builds the tenant-scoped response-cache key. tenantID must come
// from validated JWT claims, never from request input, so cross-tenant
// cache reads are structurally impossible (FEAT-006 FR-1).
func BuildKey(tenantID uuid.UUID, method, path, queryHash string) string {
	return fmt.Sprintf("%s%s:%s:%s:%s", keyPrefix, tenantID, method, path, queryHash)
}

// ResponseCache gets and sets cached downstream responses. Declared here
// (the consumer, alongside its Redis-backed implementation); CacheMiddleware
// depends on this interface.
type ResponseCache interface {
	Get(ctx context.Context, key string) (*CachedResponse, bool, error)
	Set(ctx context.Context, key string, resp *CachedResponse, ttl time.Duration) error
}

// responseStore is the subset of *redis.Client the response cache needs,
// sized to its two calls so tests can substitute a fake without a live
// Redis instance.
type responseStore interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd
}

type redisResponseCache struct {
	redis responseStore
}

// NewResponseCache returns a Redis-backed ResponseCache.
func NewResponseCache(redisClient *redis.Client) *redisResponseCache {
	return &redisResponseCache{redis: redisClient}
}

// Get implements ResponseCache. A cache miss and a Redis error are both
// reported via ok=false so callers naturally fail open by proceeding to the
// downstream call (FEAT-006 Edge Cases: Redis unavailable).
func (c *redisResponseCache) Get(ctx context.Context, key string) (*CachedResponse, bool, error) {
	raw, err := c.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("cache: get key %q: %w", key, err)
	}

	var resp CachedResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		// Corrupt cache entry: treat as a miss rather than failing the request.
		return nil, false, nil
	}

	return &resp, true, nil
}

// Set implements ResponseCache.
func (c *redisResponseCache) Set(ctx context.Context, key string, resp *CachedResponse, ttl time.Duration) error {
	encoded, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("cache: marshal response for key %q: %w", key, err)
	}
	if err := c.redis.Set(ctx, key, encoded, ttl).Err(); err != nil {
		return fmt.Errorf("cache: set key %q: %w", key, err)
	}
	return nil
}
