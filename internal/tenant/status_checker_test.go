package tenant

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

type countingRepository struct {
	tenant *Tenant
	err    error
	calls  int
}

func (f *countingRepository) GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	f.calls++
	return f.tenant, f.err
}

func (f *countingRepository) GetBySlug(ctx context.Context, slug string) (*Tenant, error) {
	return f.tenant, f.err
}

func TestMemoryStatusCache_IsActive(t *testing.T) {
	tenantID := uuid.New()
	deletedAt := time.Now()

	tests := []struct {
		name       string
		repo       *countingRepository
		wantActive bool
		wantErr    bool
	}{
		{
			name:       "active tenant",
			repo:       &countingRepository{tenant: &Tenant{ID: tenantID, IsActive: true}},
			wantActive: true,
		},
		{
			name:       "inactive tenant",
			repo:       &countingRepository{tenant: &Tenant{ID: tenantID, IsActive: false}},
			wantActive: false,
		},
		{
			name:       "soft-deleted tenant is inactive",
			repo:       &countingRepository{tenant: &Tenant{ID: tenantID, IsActive: true, DeletedAt: &deletedAt}},
			wantActive: false,
		},
		{
			name:       "tenant not found treated as inactive",
			repo:       &countingRepository{err: ErrTenantNotFound},
			wantActive: false,
		},
		{
			name:    "repository error propagates",
			repo:    &countingRepository{err: errors.New("db unavailable")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cache := NewStatusCache(tt.repo, StatusCacheTTL)
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
		})
	}
}

func TestMemoryStatusCache_RateLimits(t *testing.T) {
	tenantID := uuid.New()

	tests := []struct {
		name       string
		repo       *countingRepository
		wantLimits RateLimits
	}{
		{
			name:       "loads limits from repository",
			repo:       &countingRepository{tenant: &Tenant{ID: tenantID, RateLimitPerMinute: 60, RateLimitPerHour: 1000}},
			wantLimits: RateLimits{PerMinute: 60, PerHour: 1000},
		},
		{
			name:       "repository error does not propagate, falls back to zero-value limits",
			repo:       &countingRepository{err: errors.New("db unavailable")},
			wantLimits: RateLimits{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cache := NewStatusCache(tt.repo, StatusCacheTTL)
			limits, err := cache.RateLimits(context.Background(), tenantID)

			if err != nil {
				t.Fatalf("RateLimits() unexpected error: %v", err)
			}
			if limits != tt.wantLimits {
				t.Errorf("RateLimits() = %+v, want %+v", limits, tt.wantLimits)
			}
		})
	}
}

func TestMemoryStatusCache_SingleFetchServesBothStatusAndLimits(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	repo := &countingRepository{tenant: &Tenant{ID: tenantID, IsActive: true, RateLimitPerMinute: 60, RateLimitPerHour: 1000}}
	cache := NewStatusCache(repo, StatusCacheTTL)

	if _, err := cache.IsActive(context.Background(), tenantID); err != nil {
		t.Fatalf("IsActive() unexpected error: %v", err)
	}
	if _, err := cache.RateLimits(context.Background(), tenantID); err != nil {
		t.Fatalf("RateLimits() unexpected error: %v", err)
	}

	if repo.calls != 1 {
		t.Errorf("repo.GetByID called %d times, want 1 (status and limits should share one cached fetch)", repo.calls)
	}
}

func TestMemoryStatusCache_CachesWithinTTL(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	repo := &countingRepository{tenant: &Tenant{ID: tenantID, IsActive: true}}
	cache := NewStatusCache(repo, time.Minute)

	for i := 0; i < 3; i++ {
		if _, err := cache.IsActive(context.Background(), tenantID); err != nil {
			t.Fatalf("IsActive() unexpected error: %v", err)
		}
	}

	if repo.calls != 1 {
		t.Errorf("repo.GetByID called %d times within TTL, want 1", repo.calls)
	}
}

// slowCountingRepository is safe for concurrent GetByID calls and holds
// each one open briefly, widening the window for concurrent cache misses
// to overlap so TestMemoryStatusCache_CoalescesConcurrentMisses can
// reliably exercise the singleflight path.
type slowCountingRepository struct {
	tenant *Tenant
	calls  int64
}

func (f *slowCountingRepository) GetByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	atomic.AddInt64(&f.calls, 1)
	time.Sleep(10 * time.Millisecond)
	return f.tenant, nil
}

func (f *slowCountingRepository) GetBySlug(ctx context.Context, slug string) (*Tenant, error) {
	return f.tenant, nil
}

func TestMemoryStatusCache_CoalescesConcurrentMisses(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	repo := &slowCountingRepository{tenant: &Tenant{ID: tenantID, IsActive: true, RateLimitPerMinute: 60, RateLimitPerHour: 1000}}
	cache := NewStatusCache(repo, StatusCacheTTL)

	const concurrency = 20
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			if _, err := cache.IsActive(context.Background(), tenantID); err != nil {
				t.Errorf("IsActive() unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt64(&repo.calls); got != 1 {
		t.Errorf("repo.GetByID called %d times for %d concurrent misses, want 1", got, concurrency)
	}
}

func TestMemoryStatusCache_RefetchesAfterExpiry(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	repo := &countingRepository{tenant: &Tenant{ID: tenantID, IsActive: true}}
	cache := NewStatusCache(repo, time.Millisecond)

	if _, err := cache.IsActive(context.Background(), tenantID); err != nil {
		t.Fatalf("IsActive() unexpected error: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := cache.IsActive(context.Background(), tenantID); err != nil {
		t.Fatalf("IsActive() unexpected error: %v", err)
	}

	if repo.calls != 2 {
		t.Errorf("repo.GetByID called %d times across TTL expiry, want 2", repo.calls)
	}
}
