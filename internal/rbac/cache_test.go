package rbac

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"api-gateway/internal/database"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

// fakeRoleCacheStore is a roleCacheStore test double keyed by cache key, so
// tests can independently control the roles and permissions Get results
// (unlike tenant's fakeStatusCacheStore, which only ever needs one key at a
// time).
type fakeRoleCacheStore struct {
	getResults map[string]*redis.StringCmd
	setResult  *redis.StatusCmd
	setCalls   []struct {
		key   string
		value interface{}
	}
}

func (f *fakeRoleCacheStore) Get(ctx context.Context, key string) *redis.StringCmd {
	if cmd, ok := f.getResults[key]; ok {
		return cmd
	}
	return redis.NewStringResult("", redis.Nil)
}

func (f *fakeRoleCacheStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd {
	f.setCalls = append(f.setCalls, struct {
		key   string
		value interface{}
	}{key, value})
	if f.setResult != nil {
		return f.setResult
	}
	return redis.NewStatusResult("OK", nil)
}

// spyDBService counts GetDB() calls so tests can assert whether the
// PostgreSQL fallback path was actually taken, backed by a *sql.DB pointed
// at an unreachable address. sql.Open doesn't connect eagerly, so this is
// safe to construct without a real database — the first query against it
// fails with a genuine connection error, exercising loadRoles/loadPermissions'
// real error path.
type spyDBService struct {
	db    *sql.DB
	calls int
}

func newSpyDBService(t *testing.T) *spyDBService {
	t.Helper()
	db, err := sql.Open("pgx", "postgres://x:x@127.0.0.1:1/x?connect_timeout=1")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &spyDBService{db: db}
}

func (s *spyDBService) GetDB() *sql.DB {
	s.calls++
	return s.db
}
func (s *spyDBService) Health() database.HealthStats { return database.HealthStats{} }
func (s *spyDBService) Close() error                 { return s.db.Close() }

func sampleRoles() []Role {
	return []Role{
		{ID: uuid.New(), Name: "admin", DisplayName: "Administrator", Description: "full access", Permissions: []string{"users:read"}, IsSystemRole: true},
	}
}

func samplePermissions() []Permission {
	return []Permission{
		{ID: uuid.New(), Name: "users:read", Resource: "users", Action: "read", Description: "view users"},
	}
}

func TestLoadCache(t *testing.T) {
	roles := sampleRoles()
	permissions := samplePermissions()
	rolesJSON, err := json.Marshal(roles)
	if err != nil {
		t.Fatalf("json.Marshal(roles) error = %v", err)
	}
	permsJSON, err := json.Marshal(permissions)
	if err != nil {
		t.Fatalf("json.Marshal(permissions) error = %v", err)
	}

	tests := []struct {
		name        string
		store       *fakeRoleCacheStore
		wantDBCalls int
		wantErr     bool
	}{
		{
			name: "redis warm hit on both keys, postgres never touched",
			store: &fakeRoleCacheStore{getResults: map[string]*redis.StringCmd{
				rolesCacheKey:       redis.NewStringResult(string(rolesJSON), nil),
				permissionsCacheKey: redis.NewStringResult(string(permsJSON), nil),
			}},
			wantDBCalls: 0,
		},
		{
			name: "redis error on roles key falls back to postgres",
			store: &fakeRoleCacheStore{getResults: map[string]*redis.StringCmd{
				rolesCacheKey:       redis.NewStringResult("", errors.New("connection refused")),
				permissionsCacheKey: redis.NewStringResult(string(permsJSON), nil),
			}},
			wantDBCalls: 1,
			wantErr:     true,
		},
		{
			name: "redis error on permissions key falls back to postgres",
			store: &fakeRoleCacheStore{getResults: map[string]*redis.StringCmd{
				rolesCacheKey:       redis.NewStringResult(string(rolesJSON), nil),
				permissionsCacheKey: redis.NewStringResult("", errors.New("connection refused")),
			}},
			wantDBCalls: 1,
			wantErr:     true,
		},
		{
			name: "corrupt json on roles key falls back to postgres",
			store: &fakeRoleCacheStore{getResults: map[string]*redis.StringCmd{
				rolesCacheKey:       redis.NewStringResult("not-json", nil),
				permissionsCacheKey: redis.NewStringResult(string(permsJSON), nil),
			}},
			wantDBCalls: 1,
			wantErr:     true,
		},
		{
			name: "partial hit, permissions key missing, treated as full miss",
			store: &fakeRoleCacheStore{getResults: map[string]*redis.StringCmd{
				rolesCacheKey: redis.NewStringResult(string(rolesJSON), nil),
				// permissionsCacheKey intentionally absent: fakeRoleCacheStore
				// defaults to a redis.Nil miss for unlisted keys.
			}},
			wantDBCalls: 1,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spy := newSpyDBService(t)
			cache, err := loadCache(context.Background(), spy, tt.store, RoleCacheTTL)

			if (err != nil) != tt.wantErr {
				t.Fatalf("loadCache() error = %v, wantErr %v", err, tt.wantErr)
			}
			if spy.calls != tt.wantDBCalls {
				t.Errorf("db.GetDB() calls = %d, want %d", spy.calls, tt.wantDBCalls)
			}
			if tt.wantErr {
				if !errors.Is(err, ErrCacheLoad) {
					t.Errorf("loadCache() error = %v, want wrapping ErrCacheLoad", err)
				}
				return
			}

			if len(cache.All()) != len(roles) {
				t.Errorf("len(All()) = %d, want %d", len(cache.All()), len(roles))
			}
			if len(cache.AllPermissions()) != len(permissions) {
				t.Errorf("len(AllPermissions()) = %d, want %d", len(cache.AllPermissions()), len(permissions))
			}
			if len(tt.store.setCalls) != 0 {
				t.Errorf("redis Set() called %d times on a cache hit, want 0", len(tt.store.setCalls))
			}
			role, ok := cache.GetRole(roles[0].Name)
			if !ok || role.ID != roles[0].ID {
				t.Errorf("GetRole(%q) = %v, %v, want the redis-loaded role", roles[0].Name, role, ok)
			}
		})
	}
}

func TestRoleCache_Refresh(t *testing.T) {
	t.Run("postgres error leaves existing state untouched", func(t *testing.T) {
		existingRoles := sampleRoles()
		existingPermissions := samplePermissions()

		c := &roleCache{db: newSpyDBService(t), redis: &fakeRoleCacheStore{}, ttl: RoleCacheTTL}
		c.setState(existingRoles, existingPermissions)

		err := c.Refresh(context.Background())
		if err == nil {
			t.Fatal("Refresh() error = nil, want error")
		}
		if !errors.Is(err, ErrCacheLoad) {
			t.Errorf("Refresh() error = %v, want wrapping ErrCacheLoad", err)
		}

		if len(c.All()) != len(existingRoles) {
			t.Errorf("All() after failed Refresh() = %d roles, want %d (unchanged)", len(c.All()), len(existingRoles))
		}
		role, ok := c.GetRole(existingRoles[0].Name)
		if !ok || role.ID != existingRoles[0].ID {
			t.Errorf("GetRole(%q) after failed Refresh() = %v, %v, want the original role preserved", existingRoles[0].Name, role, ok)
		}
		if len(c.AllPermissions()) != len(existingPermissions) {
			t.Errorf("AllPermissions() after failed Refresh() = %d, want %d (unchanged)", len(c.AllPermissions()), len(existingPermissions))
		}
	})
}
