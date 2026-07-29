package rbac

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"api-gateway/internal/database"
	"api-gateway/internal/logger"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RoleCacheTTL bounds how long the Redis-cached roles/permissions snapshot
// may be served before a fresh boot falls back to PostgreSQL. Roles and
// permissions are system-defined and rarely change, so this trades a small
// staleness window for keeping most gateway replica boots off the DB path.
const RoleCacheTTL = 5 * time.Minute

const (
	rolesCacheKey       = "rbac:roles"
	permissionsCacheKey = "rbac:permissions"
)

// RoleCache resolves roles by name and lists all roles/permissions. Reads
// are served entirely from memory; PostgreSQL and Redis are only consulted
// when the cache is (re)loaded, not on every read.
type RoleCache interface {
	GetRole(name string) (*Role, bool)
	GetRoleByID(id uuid.UUID) (*Role, bool)
	All() []Role
	AllPermissions() []Permission

	// Refresh reloads roles and permissions from PostgreSQL, replacing the
	// in-memory snapshot and the Redis-cached copy. Exported for a future
	// admin-triggered refresh — nothing calls it yet. On a load error, the
	// existing in-memory cache and Redis snapshot are left untouched, so a
	// transient DB failure never degrades an already-healthy cache.
	Refresh(ctx context.Context) error
}

// roleCacheStore is the subset of *redis.Client RoleCache needs, sized to
// its two calls so tests can substitute a fake without a live Redis.
type roleCacheStore interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd
}

type roleCache struct {
	db    database.Service
	redis roleCacheStore
	ttl   time.Duration

	mu          sync.RWMutex
	roles       []Role
	byName      map[string]*Role
	byID        map[uuid.UUID]*Role
	permissions []Permission
}

// NewRoleCache loads roles and permissions into memory, preferring a warm
// Redis snapshot over PostgreSQL, and returns a RoleCache. Failing to load
// from either source is treated as a fatal startup condition by the caller
// (fail closed): authorization must never proceed against an empty or
// partial cache.
func NewRoleCache(ctx context.Context, db database.Service, redisClient *redis.Client, ttl time.Duration) (RoleCache, error) {
	// A nil *redis.Client boxed directly into the roleCacheStore interface
	// would produce a non-nil interface value (typed nil), so the `store ==
	// nil` checks in tryLoadFromRedis/writeThrough wouldn't catch it and
	// they'd panic calling Get/Set on a nil client. Normalize to a true nil
	// interface here instead.
	var store roleCacheStore
	if redisClient != nil {
		store = redisClient
	}
	return loadCache(ctx, db, store, ttl)
}

// loadCache builds a *roleCache, trying a warm Redis snapshot first and
// falling back to PostgreSQL on any miss, error, or corrupt/partial data.
// Factored out of NewRoleCache so tests can inject a fake roleCacheStore
// without a live Redis connection.
func loadCache(ctx context.Context, db database.Service, store roleCacheStore, ttl time.Duration) (*roleCache, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: nil database service", ErrCacheLoad)
	}

	c := &roleCache{db: db, redis: store, ttl: ttl}

	if roles, permissions, ok := tryLoadFromRedis(ctx, store); ok {
		c.setState(roles, permissions)
		logger.FromContext(ctx).Info("rbac: loaded role cache from redis", "roles", len(roles), "permissions", len(permissions))
		return c, nil
	}

	roles, permissions, err := loadFromPostgres(ctx, db)
	if err != nil {
		return nil, err
	}
	c.setState(roles, permissions)
	writeThrough(ctx, store, ttl, roles, permissions)

	logger.FromContext(ctx).Info("rbac: loaded role cache", "roles", len(roles), "permissions", len(permissions))
	return c, nil
}

// tryLoadFromRedis reports a cache hit only when both the roles and
// permissions keys are present and unmarshal cleanly — a partial hit is
// treated as a full miss so the cache is never built from inconsistent
// data.
func tryLoadFromRedis(ctx context.Context, store roleCacheStore) ([]Role, []Permission, bool) {
	if store == nil {
		return nil, nil, false
	}

	rolesRaw, err := store.Get(ctx, rolesCacheKey).Bytes()
	if err != nil {
		return nil, nil, false
	}
	permsRaw, err := store.Get(ctx, permissionsCacheKey).Bytes()
	if err != nil {
		return nil, nil, false
	}

	var roles []Role
	if err := json.Unmarshal(rolesRaw, &roles); err != nil {
		return nil, nil, false
	}
	var permissions []Permission
	if err := json.Unmarshal(permsRaw, &permissions); err != nil {
		return nil, nil, false
	}

	return roles, permissions, true
}

// writeThrough best-effort populates Redis with a freshly loaded snapshot.
// Failures are logged, not returned: Redis is a cache, not the source of
// truth, so a write failure must never fail a successful PostgreSQL load.
func writeThrough(ctx context.Context, store roleCacheStore, ttl time.Duration, roles []Role, permissions []Permission) {
	if store == nil {
		return
	}

	if encoded, err := json.Marshal(roles); err != nil {
		logger.FromContext(ctx).Warn("rbac: failed to marshal roles for redis cache", "error", err.Error())
	} else if err := store.Set(ctx, rolesCacheKey, encoded, ttl).Err(); err != nil {
		logger.FromContext(ctx).Warn("rbac: failed to write role cache to redis", "key", rolesCacheKey, "error", err.Error())
	}

	if encoded, err := json.Marshal(permissions); err != nil {
		logger.FromContext(ctx).Warn("rbac: failed to marshal permissions for redis cache", "error", err.Error())
	} else if err := store.Set(ctx, permissionsCacheKey, encoded, ttl).Err(); err != nil {
		logger.FromContext(ctx).Warn("rbac: failed to write permission cache to redis", "key", permissionsCacheKey, "error", err.Error())
	}
}

// loadFromPostgres loads roles and permissions from the database, wrapping
// either failure in ErrCacheLoad.
func loadFromPostgres(ctx context.Context, db database.Service) ([]Role, []Permission, error) {
	roles, err := loadRoles(ctx, db.GetDB())
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrCacheLoad, err)
	}

	permissions, err := loadPermissions(ctx, db.GetDB())
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrCacheLoad, err)
	}

	return roles, permissions, nil
}

// setState rebuilds the byName/byID indices and swaps them, along with the
// roles/permissions slices, into the cache under the write lock.
func (c *roleCache) setState(roles []Role, permissions []Permission) {
	byName := make(map[string]*Role, len(roles))
	byID := make(map[uuid.UUID]*Role, len(roles))
	for i := range roles {
		byName[roles[i].Name] = &roles[i]
		byID[roles[i].ID] = &roles[i]
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.roles = roles
	c.byName = byName
	c.byID = byID
	c.permissions = permissions
}

// Refresh implements RoleCache.
func (c *roleCache) Refresh(ctx context.Context) error {
	roles, permissions, err := loadFromPostgres(ctx, c.db)
	if err != nil {
		return err
	}

	c.setState(roles, permissions)
	writeThrough(ctx, c.redis, c.ttl, roles, permissions)

	logger.FromContext(ctx).Info("rbac: refreshed role cache", "roles", len(roles), "permissions", len(permissions))
	return nil
}

// GetRole implements RoleCache.
func (c *roleCache) GetRole(name string) (*Role, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	role, ok := c.byName[name]
	return role, ok
}

// GetRoleByID implements RoleCache.
func (c *roleCache) GetRoleByID(id uuid.UUID) (*Role, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	role, ok := c.byID[id]
	return role, ok
}

// All implements RoleCache.
func (c *roleCache) All() []Role {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.roles
}

// AllPermissions implements RoleCache.
func (c *roleCache) AllPermissions() []Permission {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.permissions
}

func loadRoles(ctx context.Context, db *sql.DB) ([]Role, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, display_name, description, permissions, is_system_role, created_at, updated_at
		FROM roles
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("query roles: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.FromContext(ctx).Error("rbac: failed to close roles rows", "error", cerr.Error())
		}
	}()

	var roles []Role
	for rows.Next() {
		var (
			role      Role
			id        uuid.UUID
			permsJSON []byte
		)
		if err := rows.Scan(&id, &role.Name, &role.DisplayName, &role.Description, &permsJSON, &role.IsSystemRole, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		role.ID = id
		permissions, err := unmarshalPermissions(permsJSON)
		if err != nil {
			return nil, fmt.Errorf("unmarshal role %q permissions: %w", role.Name, err)
		}
		role.Permissions = permissions
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate roles: %w", err)
	}

	return roles, nil
}

// loadPermissions returns every row in the permissions table.
func loadPermissions(ctx context.Context, db *sql.DB) ([]Permission, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, resource, action, description, created_at
		FROM permissions
		ORDER BY resource, action
	`)
	if err != nil {
		return nil, fmt.Errorf("query permissions: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			logger.FromContext(ctx).Error("rbac: failed to close permissions rows", "error", cerr.Error())
		}
	}()

	var permissions []Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Resource, &p.Action, &p.Description, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		permissions = append(permissions, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate permissions: %w", err)
	}

	return permissions, nil
}

// unmarshalPermissions decodes a role's permissions JSONB column (a JSON
// array of "resource:action" strings) into a []string.
func unmarshalPermissions(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var permissions []string
	if err := json.Unmarshal(raw, &permissions); err != nil {
		return nil, fmt.Errorf("unmarshal permissions json: %w", err)
	}
	return permissions, nil
}
