package rbac

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"api-gateway/internal/database"
	"api-gateway/internal/logger"

	"github.com/google/uuid"
)

// RoleCache resolves roles by name and lists all roles/permissions, served
// entirely from memory once loaded. Roles and permissions are system-defined
// and immutable for the MVP, so the cache is populated once at startup
// rather than refreshed per request.
type RoleCache interface {
	GetRole(name string) (*Role, bool)
	GetRoleByID(id uuid.UUID) (*Role, bool)
	All() []Role
	AllPermissions() []Permission
}

type roleCache struct {
	roles       []Role
	byName      map[string]*Role
	byID        map[uuid.UUID]*Role
	permissions []Permission
}

// NewRoleCache loads roles and permissions from PostgreSQL into memory once
// and returns a RoleCache. Failing to load is treated as a fatal startup
// condition by the caller (fail closed): authorization must never proceed
// against an empty or partial cache.
func NewRoleCache(ctx context.Context, db database.Service) (RoleCache, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: nil database service", ErrCacheLoad)
	}

	roles, err := loadRoles(ctx, db.GetDB())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCacheLoad, err)
	}

	permissions, err := loadPermissions(ctx, db.GetDB())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCacheLoad, err)
	}

	byName := make(map[string]*Role, len(roles))
	byID := make(map[uuid.UUID]*Role, len(roles))
	for i := range roles {
		byName[roles[i].Name] = &roles[i]
		byID[roles[i].ID] = &roles[i]
	}

	logger.FromContext(ctx).Info("rbac: loaded role cache", "roles", len(roles), "permissions", len(permissions))
	return &roleCache{roles: roles, byName: byName, byID: byID, permissions: permissions}, nil
}

// GetRole implements RoleCache.
func (c *roleCache) GetRole(name string) (*Role, bool) {
	role, ok := c.byName[name]
	return role, ok
}

// GetRoleByID implements RoleCache.
func (c *roleCache) GetRoleByID(id uuid.UUID) (*Role, bool) {
	role, ok := c.byID[id]
	return role, ok
}

// All implements RoleCache.
func (c *roleCache) All() []Role {
	return c.roles
}

// AllPermissions implements RoleCache.
func (c *roleCache) AllPermissions() []Permission {
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
