package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type fakeLimiterStore struct {
	val interface{}
	err error
}

func (f *fakeLimiterStore) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	cmd := redis.NewCmd(ctx)
	if f.err != nil {
		cmd.SetErr(f.err)
		return cmd
	}
	cmd.SetVal(f.val)
	return cmd
}

func TestSlidingWindowLimiter_Allow(t *testing.T) {
	tenantID, userID := uuid.New(), uuid.New()
	fixedNow := time.Date(2026, 1, 1, 0, 0, 30, 0, time.UTC) // 30s into the minute bucket

	tests := []struct {
		name       string
		store      *fakeLimiterStore
		limit      int
		wantErr    bool
		wantAllow  bool
		wantRemain int
	}{
		{
			name:       "well within limit",
			store:      &fakeLimiterStore{val: []interface{}{int64(5), "0"}},
			limit:      60,
			wantAllow:  true,
			wantRemain: 55,
		},
		{
			name:      "over limit denies",
			store:     &fakeLimiterStore{val: []interface{}{int64(61), "0"}},
			limit:     60,
			wantAllow: false,
		},
		{
			name: "weighted previous bucket count counts toward the limit",
			// weight at 30s into a 60s bucket is 0.5, so weighted count is
			// 20 (current) + 40*0.5 (previous) = 40.
			store:      &fakeLimiterStore{val: []interface{}{int64(20), "40"}},
			limit:      60,
			wantAllow:  true,
			wantRemain: 20,
		},
		{
			name: "weighted previous bucket count pushes over limit",
			// weighted count is 40 + 40*0.5 = 60, exactly at the limit: still allowed (<=).
			store:      &fakeLimiterStore{val: []interface{}{int64(41), "40"}},
			limit:      60,
			wantAllow:  false,
			wantRemain: 0,
		},
		{
			name:    "redis eval error propagates",
			store:   &fakeLimiterStore{err: errors.New("connection refused")},
			limit:   60,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			limiter := &SlidingWindowLimiter{redis: tt.store, now: func() time.Time { return fixedNow }}
			decision, err := limiter.Allow(context.Background(), tenantID, userID, WindowMinute, tt.limit)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Allow() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if decision.Allowed != tt.wantAllow {
				t.Errorf("Allow() allowed = %v, want %v", decision.Allowed, tt.wantAllow)
			}
			if decision.Remaining != tt.wantRemain {
				t.Errorf("Allow() remaining = %d, want %d", decision.Remaining, tt.wantRemain)
			}
		})
	}
}

func TestSlidingWindowLimiter_Allow_UnknownWindow(t *testing.T) {
	t.Parallel()

	limiter := NewSlidingWindowLimiter(nil)
	_, err := limiter.Allow(context.Background(), uuid.New(), uuid.New(), Window("day"), 10)
	if err == nil {
		t.Fatal("Allow() with unknown window: want error, got nil")
	}
}
