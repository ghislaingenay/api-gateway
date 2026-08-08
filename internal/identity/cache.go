package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"api-gateway/internal/logger"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// tenantUserStore is the subset of *redis.Client TenantUserCache needs,
// sized to its two calls so tests can substitute a fake without a live
// Redis.
type tenantUserStore interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd
}

// tenantUserLoader is the subset of Resolver TenantUserCache needs on a
// cache miss.
type tenantUserLoader interface {
	ResolveTenantUser(ctx context.Context, userID, tenantID uuid.UUID) (*TenantUser, error)
}

// cachedTenantUser is the wire format stored in both cache tiers. Found is
// false for a negative result (caller verified not to be a tenant member),
// cached just like a hit so repeated 403s for the same pair don't
// repeatedly hit PostgreSQL.
type cachedTenantUser struct {
	Found  bool      `json:"found"`
	RoleID uuid.UUID `json:"role_id,omitempty"`
}

type entry struct {
	value     cachedTenantUser
	expiresAt time.Time
}

// TenantUserCache resolves a caller's tenant_users role assignment, cached
// in-process and (if a Redis client is configured) in Redis, to keep
// per-request resolution off PostgreSQL (TD-012 §8). Concurrent
// misses/expiries for the same (sub, tenant) pair are collapsed into a
// single load via singleflight, following the same pattern
// tenant.memoryStatusCache already establishes.
type TenantUserCache struct {
	loader tenantUserLoader
	redis  tenantUserStore
	ttl    time.Duration

	mu   sync.RWMutex
	data map[string]entry

	group singleflight.Group

	stopCleanup chan struct{}
}

// NewTenantUserCache returns a *TenantUserCache backed by loader. A nil
// redisClient disables the Redis tier (in-process caching only), which
// tests rely on. Call Close when the cache is no longer needed to stop
// the background expiry sweep.
func NewTenantUserCache(loader Resolver, redisClient *redis.Client, ttl time.Duration) *TenantUserCache {
	var store tenantUserStore
	if redisClient != nil {
		store = redisClient
	}
	c := &TenantUserCache{
		loader:      loader,
		redis:       store,
		ttl:         ttl,
		data:        make(map[string]entry),
		stopCleanup: make(chan struct{}),
	}
	if ttl > 0 {
		go c.cleanupLoop()
	}
	return c
}

// Close stops the background expiry sweep. Safe to call once.
func (c *TenantUserCache) Close() {
	close(c.stopCleanup)
}

func (c *TenantUserCache) cleanupLoop() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-c.stopCleanup:
			return
		}
	}
}

func (c *TenantUserCache) deleteExpired() {
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.data {
		if now.After(e.expiresAt) {
			delete(c.data, k)
		}
	}
	c.mu.Unlock()
}

// Resolve returns the caller's TenantUser role assignment for tenantID, or
// ErrNotMember if keycloakSub has no verified tenant_users row there.
func (c *TenantUserCache) Resolve(ctx context.Context, keycloakSub string, userID, tenantID uuid.UUID) (*TenantUser, error) {
	key := cacheKey(keycloakSub, tenantID)

	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if ok && time.Now().Before(e.expiresAt) {
		return toTenantUser(userID, e.value)
	}

	// Note: the first caller to arrive owns the context used for the shared
	// fetch — if it's canceled, every other caller waiting on this key is
	// canceled with it. Acceptable here since the load is a single fast
	// lookup and callers are all requesting the same data.
	v, err, _ := c.group.Do(key, func() (interface{}, error) {
		return c.load(ctx, key, userID, tenantID)
	})
	if err != nil {
		return nil, err
	}
	return toTenantUser(userID, v.(cachedTenantUser))
}

func (c *TenantUserCache) load(ctx context.Context, key string, userID, tenantID uuid.UUID) (cachedTenantUser, error) {
	if ctu, ok := c.tryRedis(ctx, key); ok {
		c.setLocal(key, ctu)
		return ctu, nil
	}

	tu, err := c.loader.ResolveTenantUser(ctx, userID, tenantID)
	var ctu cachedTenantUser
	switch {
	case err == nil:
		ctu = cachedTenantUser{Found: true, RoleID: tu.RoleID}
	case errors.Is(err, ErrNotMember):
		ctu = cachedTenantUser{Found: false}
	default:
		return cachedTenantUser{}, fmt.Errorf("load tenant user: %w", err)
	}

	c.setLocal(key, ctu)
	c.writeThrough(ctx, key, ctu)
	return ctu, nil
}

func (c *TenantUserCache) tryRedis(ctx context.Context, key string) (cachedTenantUser, bool) {
	if c.redis == nil {
		return cachedTenantUser{}, false
	}
	raw, err := c.redis.Get(ctx, key).Bytes()
	if err != nil {
		return cachedTenantUser{}, false
	}
	var ctu cachedTenantUser
	if err := json.Unmarshal(raw, &ctu); err != nil {
		return cachedTenantUser{}, false
	}
	return ctu, true
}

// writeThrough best-effort populates Redis. Failures are logged, not
// returned: Redis is a cache, not the source of truth, so a write failure
// must never fail a successful PostgreSQL load.
func (c *TenantUserCache) writeThrough(ctx context.Context, key string, ctu cachedTenantUser) {
	if c.redis == nil {
		return
	}
	encoded, err := json.Marshal(ctu)
	if err != nil {
		logger.FromContext(ctx).Warn("identity: failed to marshal tenant user for redis cache", "error", err.Error())
		return
	}
	if err := c.redis.Set(ctx, key, encoded, c.ttl).Err(); err != nil {
		logger.FromContext(ctx).Warn("identity: failed to write tenant user cache to redis", "error", err.Error())
	}
}

func (c *TenantUserCache) setLocal(key string, ctu cachedTenantUser) {
	c.mu.Lock()
	c.data[key] = entry{value: ctu, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func toTenantUser(userID uuid.UUID, ctu cachedTenantUser) (*TenantUser, error) {
	if !ctu.Found {
		return nil, ErrNotMember
	}
	return &TenantUser{UserID: userID, RoleID: ctu.RoleID}, nil
}

func cacheKey(keycloakSub string, tenantID uuid.UUID) string {
	return fmt.Sprintf("identity:tenant_user:%s:%s", keycloakSub, tenantID.String())
}
