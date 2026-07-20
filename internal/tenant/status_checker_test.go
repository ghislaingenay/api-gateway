package tenant

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type fakeRepository struct {
	tenant *Tenant
	err    error
}

func (f *fakeRepository) GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	return f.tenant, f.err
}

type fakeStatusCacheStore struct {
	getResult *redis.StringCmd
	setResult *redis.StatusCmd
	setCalls  []struct {
		key   string
		value interface{}
	}
}

func (f *fakeStatusCacheStore) Get(ctx context.Context, key string) *redis.StringCmd {
	return f.getResult
}

func (f *fakeStatusCacheStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd {
	f.setCalls = append(f.setCalls, struct {
		key   string
		value interface{}
	}{key, value})
	return f.setResult
}

func TestRedisStatusCache_IsActive(t *testing.T) {
	tenantID := uuid.New()
	deletedAt := time.Now()

	tests := []struct {
		name       string
		store      *fakeStatusCacheStore
		repo       *fakeRepository
		wantActive bool
		wantErr    bool
		wantWrite  bool
	}{
		{
			name:       "cache hit active",
			store:      &fakeStatusCacheStore{getResult: redis.NewStringResult(statusActive, nil)},
			wantActive: true,
		},
		{
			name:       "cache hit inactive",
			store:      &fakeStatusCacheStore{getResult: redis.NewStringResult(statusInactive, nil)},
			wantActive: false,
		},
		{
			name:  "cache miss, tenant active, populates cache",
			store: &fakeStatusCacheStore{getResult: redis.NewStringResult("", redis.Nil), setResult: redis.NewStatusResult("OK", nil)},
			repo: &fakeRepository{
				tenant: &Tenant{ID: tenantID, IsActive: true},
			},
			wantActive: true,
			wantWrite:  true,
		},
		{
			name:  "cache miss, tenant inactive",
			store: &fakeStatusCacheStore{getResult: redis.NewStringResult("", redis.Nil), setResult: redis.NewStatusResult("OK", nil)},
			repo: &fakeRepository{
				tenant: &Tenant{ID: tenantID, IsActive: false},
			},
			wantActive: false,
			wantWrite:  true,
		},
		{
			name:  "cache miss, tenant soft-deleted",
			store: &fakeStatusCacheStore{getResult: redis.NewStringResult("", redis.Nil), setResult: redis.NewStatusResult("OK", nil)},
			repo: &fakeRepository{
				tenant: &Tenant{ID: tenantID, IsActive: true, DeletedAt: &deletedAt},
			},
			wantActive: false,
			wantWrite:  true,
		},
		{
			name:  "cache miss, tenant not found treated as inactive",
			store: &fakeStatusCacheStore{getResult: redis.NewStringResult("", redis.Nil), setResult: redis.NewStatusResult("OK", nil)},
			repo: &fakeRepository{
				err: ErrTenantNotFound,
			},
			wantActive: false,
			wantErr:    false,
		},
		{
			name:  "cache miss, repository error propagates",
			store: &fakeStatusCacheStore{getResult: redis.NewStringResult("", redis.Nil)},
			repo: &fakeRepository{
				err: errors.New("db unavailable"),
			},
			wantErr: true,
		},
		{
			name:  "redis error falls back to repository (best-effort cache)",
			store: &fakeStatusCacheStore{getResult: redis.NewStringResult("", errors.New("connection refused")), setResult: redis.NewStatusResult("OK", nil)},
			repo: &fakeRepository{
				tenant: &Tenant{ID: tenantID, IsActive: true},
			},
			wantActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cache := &redisStatusCache{repo: tt.repo, redis: tt.store, ttl: StatusCacheTTL}
			active, err := cache.IsActive(context.Background(), tenantID)

			if (err != nil) != tt.wantErr {
				t.Fatalf("IsActive() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if active != tt.wantActive {
				t.Errorf("IsActive() = %v, want %v", active, tt.wantActive)
			}
			if tt.wantWrite && len(tt.store.setCalls) != 1 {
				t.Errorf("expected cache write, got %d writes", len(tt.store.setCalls))
			}
		})
	}
}

func TestRedisStatusCache_RateLimits(t *testing.T) {
	tenantID := uuid.New()

	tests := []struct {
		name       string
		store      *fakeStatusCacheStore
		repo       *fakeRepository
		wantLimits RateLimits
		wantErr    bool
		wantWrite  bool
	}{
		{
			name:       "cache hit returns cached limits",
			store:      &fakeStatusCacheStore{getResult: redis.NewStringResult(`{"PerMinute":30,"PerHour":500}`, nil)},
			wantLimits: RateLimits{PerMinute: 30, PerHour: 500},
		},
		{
			name:  "cache miss loads from repository and populates cache",
			store: &fakeStatusCacheStore{getResult: redis.NewStringResult("", redis.Nil), setResult: redis.NewStatusResult("OK", nil)},
			repo: &fakeRepository{
				tenant: &Tenant{ID: tenantID, RateLimitPerMinute: 60, RateLimitPerHour: 1000},
			},
			wantLimits: RateLimits{PerMinute: 60, PerHour: 1000},
			wantWrite:  true,
		},
		{
			name:  "corrupt cache entry falls back to repository",
			store: &fakeStatusCacheStore{getResult: redis.NewStringResult("not-json", nil), setResult: redis.NewStatusResult("OK", nil)},
			repo: &fakeRepository{
				tenant: &Tenant{ID: tenantID, RateLimitPerMinute: 60, RateLimitPerHour: 1000},
			},
			wantLimits: RateLimits{PerMinute: 60, PerHour: 1000},
			wantWrite:  true,
		},
		{
			name:  "repository error does not propagate, falls back to zero-value limits",
			store: &fakeStatusCacheStore{getResult: redis.NewStringResult("", redis.Nil)},
			repo: &fakeRepository{
				err: errors.New("db unavailable"),
			},
			wantLimits: RateLimits{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cache := &redisStatusCache{repo: tt.repo, redis: tt.store, ttl: StatusCacheTTL}
			limits, err := cache.RateLimits(context.Background(), tenantID)

			if (err != nil) != tt.wantErr {
				t.Fatalf("RateLimits() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if limits != tt.wantLimits {
				t.Errorf("RateLimits() = %+v, want %+v", limits, tt.wantLimits)
			}
			if tt.wantWrite && len(tt.store.setCalls) != 1 {
				t.Errorf("expected cache write, got %d writes", len(tt.store.setCalls))
			}
		})
	}
}
