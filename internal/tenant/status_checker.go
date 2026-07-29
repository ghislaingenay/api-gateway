package tenant

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"api-gateway/internal/logger"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

// StatusCacheTTL bounds how long a cached tenant entry (active status and
// rate limits together) may be served before the next read refetches it
// from PostgreSQL. Tenant status/limits change rarely, so this trades a
// small staleness window for keeping both off the request's hot path
// entirely, rather than round-tripping to Redis on every request.
const StatusCacheTTL = 30 * time.Second

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

// entry is the cached snapshot for one tenant: active status and rate
// limits loaded together from a single PostgreSQL read, so IsActive and
// RateLimits never trigger two separate loads for the same tenant.
type entry struct {
	active    bool
	limits    RateLimits
	expiresAt time.Time
}

// memoryStatusCache satisfies both StatusChecker and RateLimitProvider from
// a process-local, per-tenant cache instead of a shared Redis cache. Going
// to Redis for this put a network round trip (sometimes cross-region) on
// every request for data that changes rarely; keeping the last-known value
// in memory removes that round trip for the common case. The trade-off is
// that each replica refetches independently on its own TTL rather than
// sharing one warm cache, which is acceptable given how infrequently tenant
// status/limits actually change.
type memoryStatusCache struct {
	repo Repository
	ttl  time.Duration

	mu   sync.RWMutex
	data map[uuid.UUID]entry

	// group collapses concurrent loads for the same tenant into a single
	// PostgreSQL query, so a burst of requests arriving right as an entry
	// expires doesn't turn into a burst of duplicate DB reads.
	group singleflight.Group
}

// NewStatusCache returns a StatusChecker/RateLimitProvider backed by an
// in-process cache, falling back to repo on a miss or expiry and
// populating the cache with the result. A tenant that no longer exists is
// cached as inactive rather than retried every call, and is reported as
// inactive rather than as an error, so gateway callers fail closed (403)
// instead of erroring (500).
func NewStatusCache(repo Repository, ttl time.Duration) *memoryStatusCache {
	return &memoryStatusCache{repo: repo, ttl: ttl, data: make(map[uuid.UUID]entry)}
}

// IsActive implements StatusChecker.
func (c *memoryStatusCache) IsActive(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	e, err := c.get(ctx, tenantID)
	if err != nil {
		return false, err
	}
	return e.active, nil
}

// RateLimits implements RateLimitProvider.
func (c *memoryStatusCache) RateLimits(ctx context.Context, tenantID uuid.UUID) (RateLimits, error) {
	e, err := c.get(ctx, tenantID)
	if err != nil {
		// Don't propagate a Postgres failure as a rate-limit error: doing so
		// would make the caller fail open (skip rate limiting entirely) for
		// a DB outage, not just a cache miss. Log it and fall back to a
		// zero-value RateLimits instead, which callers resolve against their
		// own configured defaults, so rate limiting still applies.
		logger.FromContext(ctx).Warn("tenant: failed to load rate limits, falling back to defaults",
			"tenant_id", tenantID.String(),
			"error", err.Error(),
		)
		return RateLimits{}, nil
	}
	return e.limits, nil
}

// get returns the cached entry for tenantID, refetching from PostgreSQL
// when absent or past its TTL. Concurrent misses/expiries for the same
// tenant share one in-flight fetch via c.group rather than each issuing
// their own query.
func (c *memoryStatusCache) get(ctx context.Context, tenantID uuid.UUID) (entry, error) {
	c.mu.RLock()
	e, ok := c.data[tenantID]
	c.mu.RUnlock()
	if ok && time.Now().Before(e.expiresAt) {
		return e, nil
	}

	// Note: the first caller to arrive owns the context used for the
	// shared fetch — if it's canceled, every other caller waiting on this
	// key is canceled with it. Acceptable here since the load is a single
	// fast primary-key read and callers are all requesting the same data.
	v, err, _ := c.group.Do(tenantID.String(), func() (interface{}, error) {
		return c.load(ctx, tenantID)
	})
	if err != nil {
		return entry{}, err
	}
	return v.(entry), nil
}

// load fetches tenantID from PostgreSQL and populates the cache, called at
// most once per outstanding miss/expiry via c.group.
func (c *memoryStatusCache) load(ctx context.Context, tenantID uuid.UUID) (entry, error) {
	t, err := c.repo.GetByID(ctx, tenantID)
	if err != nil {
		if errors.Is(err, ErrTenantNotFound) {
			e := entry{active: false, expiresAt: time.Now().Add(c.ttl)}
			c.set(tenantID, e)
			return e, nil
		}
		return entry{}, fmt.Errorf("load tenant for status check: %w", err)
	}

	e := entry{
		active:    t.IsActive && t.DeletedAt == nil,
		limits:    RateLimits{PerMinute: t.RateLimitPerMinute, PerHour: t.RateLimitPerHour},
		expiresAt: time.Now().Add(c.ttl),
	}
	c.set(tenantID, e)
	return e, nil
}

func (c *memoryStatusCache) set(tenantID uuid.UUID, e entry) {
	c.mu.Lock()
	c.data[tenantID] = e
	c.mu.Unlock()
}
