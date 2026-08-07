package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type fakeTenantUserStore struct {
	getResults map[string]*redis.StringCmd
	setCalls   []string
}

func (f *fakeTenantUserStore) Get(ctx context.Context, key string) *redis.StringCmd {
	if cmd, ok := f.getResults[key]; ok {
		return cmd
	}
	return redis.NewStringResult("", redis.Nil)
}

func (f *fakeTenantUserStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd {
	f.setCalls = append(f.setCalls, key)
	return redis.NewStatusResult("OK", nil)
}

type spyResolver struct {
	membership *TenantUser
	err        error
	calls      int
}

func (s *spyResolver) EnsureUser(ctx context.Context, keycloakSub, email string) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (s *spyResolver) ResolveTenantUser(ctx context.Context, userID, tenantID uuid.UUID) (*TenantUser, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.membership, nil
}

func TestTenantUserCache_Resolve_CacheMissFallsBackToLoader(t *testing.T) {
	t.Parallel()

	userID, tenantID, roleID := uuid.New(), uuid.New(), uuid.New()
	loader := &spyResolver{membership: &TenantUser{UserID: userID, RoleID: roleID}}
	cache := NewTenantUserCache(loader, nil, time.Minute)

	tu, err := cache.Resolve(context.Background(), "sub-1", userID, tenantID)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if tu.RoleID != roleID {
		t.Errorf("RoleID = %v, want %v", tu.RoleID, roleID)
	}
	if loader.calls != 1 {
		t.Errorf("loader called %d times, want 1", loader.calls)
	}
}

func TestTenantUserCache_Resolve_LocalCacheHitSkipsLoader(t *testing.T) {
	t.Parallel()

	userID, tenantID, roleID := uuid.New(), uuid.New(), uuid.New()
	loader := &spyResolver{membership: &TenantUser{UserID: userID, RoleID: roleID}}
	cache := NewTenantUserCache(loader, nil, time.Minute)

	if _, err := cache.Resolve(context.Background(), "sub-1", userID, tenantID); err != nil {
		t.Fatalf("first Resolve() error = %v", err)
	}
	if _, err := cache.Resolve(context.Background(), "sub-1", userID, tenantID); err != nil {
		t.Fatalf("second Resolve() error = %v", err)
	}

	if loader.calls != 1 {
		t.Errorf("loader called %d times, want 1 (second call should hit local cache)", loader.calls)
	}
}

func TestTenantUserCache_Resolve_NegativeCaching(t *testing.T) {
	t.Parallel()

	userID, tenantID := uuid.New(), uuid.New()
	loader := &spyResolver{err: ErrNotMember}
	cache := NewTenantUserCache(loader, nil, time.Minute)

	_, err := cache.Resolve(context.Background(), "sub-1", userID, tenantID)
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("Resolve() error = %v, want ErrNotMember", err)
	}

	_, err = cache.Resolve(context.Background(), "sub-1", userID, tenantID)
	if !errors.Is(err, ErrNotMember) {
		t.Fatalf("second Resolve() error = %v, want ErrNotMember", err)
	}

	if loader.calls != 1 {
		t.Errorf("loader called %d times, want 1 (negative result should be cached)", loader.calls)
	}
}

func TestTenantUserCache_Resolve_LoaderErrorPropagates(t *testing.T) {
	t.Parallel()

	userID, tenantID := uuid.New(), uuid.New()
	loader := &spyResolver{err: errors.New("db unavailable")}
	cache := NewTenantUserCache(loader, nil, time.Minute)

	_, err := cache.Resolve(context.Background(), "sub-1", userID, tenantID)
	if err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
	if errors.Is(err, ErrNotMember) {
		t.Error("expected a load error, not ErrNotMember")
	}
}

func TestTenantUserCache_Resolve_TTLExpiryReloads(t *testing.T) {
	t.Parallel()

	userID, tenantID, roleID := uuid.New(), uuid.New(), uuid.New()
	loader := &spyResolver{membership: &TenantUser{UserID: userID, RoleID: roleID}}
	cache := NewTenantUserCache(loader, nil, -time.Second) // already expired

	if _, err := cache.Resolve(context.Background(), "sub-1", userID, tenantID); err != nil {
		t.Fatalf("first Resolve() error = %v", err)
	}
	if _, err := cache.Resolve(context.Background(), "sub-1", userID, tenantID); err != nil {
		t.Fatalf("second Resolve() error = %v", err)
	}

	if loader.calls != 2 {
		t.Errorf("loader called %d times, want 2 (expired entry should reload)", loader.calls)
	}
}

func TestTenantUserCache_Resolve_RedisHitSkipsLoader(t *testing.T) {
	t.Parallel()

	userID, tenantID, roleID := uuid.New(), uuid.New(), uuid.New()
	loader := &spyResolver{err: errors.New("should not be called")}
	store := &fakeTenantUserStore{getResults: map[string]*redis.StringCmd{
		cacheKey("sub-1", tenantID): redis.NewStringResult(`{"found":true,"role_id":"`+roleID.String()+`"}`, nil),
	}}

	cache := &TenantUserCache{loader: loader, redis: store, ttl: time.Minute, data: make(map[string]entry)}

	tu, err := cache.Resolve(context.Background(), "sub-1", userID, tenantID)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if tu.RoleID != roleID {
		t.Errorf("RoleID = %v, want %v", tu.RoleID, roleID)
	}
	if loader.calls != 0 {
		t.Errorf("loader called %d times, want 0 (redis hit should skip it)", loader.calls)
	}
}

func TestTenantUserCache_Resolve_WritesThroughToRedisOnLoad(t *testing.T) {
	t.Parallel()

	userID, tenantID, roleID := uuid.New(), uuid.New(), uuid.New()
	loader := &spyResolver{membership: &TenantUser{UserID: userID, RoleID: roleID}}
	store := &fakeTenantUserStore{}
	cache := &TenantUserCache{loader: loader, redis: store, ttl: time.Minute, data: make(map[string]entry)}

	if _, err := cache.Resolve(context.Background(), "sub-1", userID, tenantID); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(store.setCalls) != 1 {
		t.Errorf("redis Set() called %d times, want 1", len(store.setCalls))
	}
}
