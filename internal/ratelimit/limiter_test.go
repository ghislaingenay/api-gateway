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

func TestSlidingWindowLimiter_AllowKey(t *testing.T) {
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
			// weight at 30s into a 60s bucket is 0.5, so weighted count is
			// 41 (current) + 40*0.5 (previous) = 61, over the limit: denied.
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
			decision, err := limiter.AllowKey(context.Background(), "key", WindowMinute, tt.limit)

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

func TestSlidingWindowLimiter_AllowKey_UnknownWindow(t *testing.T) {
	t.Parallel()

	limiter := NewSlidingWindowLimiter(nil)
	_, err := limiter.AllowKey(context.Background(), "key", Window("day"), 10)
	if err == nil {
		t.Fatal("AllowKey() with unknown window: want error, got nil")
	}
}

func TestSlidingWindowLimiter_AllowBoth(t *testing.T) {
	tenantID, userID := uuid.New(), uuid.New()
	fixedNow := time.Date(2026, 1, 1, 0, 0, 30, 0, time.UTC) // 30s into the minute bucket, well within the hour bucket

	tests := []struct {
		name           string
		store          *fakeLimiterStore
		minuteLimit    int
		hourLimit      int
		wantErr        bool
		wantMinuteOK   bool
		wantHourOK     bool
		wantMinuteRem  int
		wantHourRemain int
	}{
		{
			name:           "well within both limits",
			store:          &fakeLimiterStore{val: []interface{}{int64(5), "0", int64(50), "0"}},
			minuteLimit:    60,
			hourLimit:      1000,
			wantMinuteOK:   true,
			wantHourOK:     true,
			wantMinuteRem:  55,
			wantHourRemain: 950,
		},
		{
			name:           "minute over limit denies minute only",
			store:          &fakeLimiterStore{val: []interface{}{int64(61), "0", int64(50), "0"}},
			minuteLimit:    60,
			hourLimit:      1000,
			wantHourOK:     true,
			wantHourRemain: 950,
		},
		{
			name:    "redis eval error propagates",
			store:   &fakeLimiterStore{err: errors.New("connection refused")},
			wantErr: true,
		},
		{
			name:    "malformed eval result shape errors",
			store:   &fakeLimiterStore{val: []interface{}{int64(1), "0"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			limiter := &SlidingWindowLimiter{redis: tt.store, now: func() time.Time { return fixedNow }}
			minute, hour, err := limiter.AllowBoth(context.Background(), tenantID, userID, tt.minuteLimit, tt.hourLimit)

			if (err != nil) != tt.wantErr {
				t.Fatalf("AllowBoth() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if minute.Allowed != tt.wantMinuteOK {
				t.Errorf("minute.Allowed = %v, want %v", minute.Allowed, tt.wantMinuteOK)
			}
			if hour.Allowed != tt.wantHourOK {
				t.Errorf("hour.Allowed = %v, want %v", hour.Allowed, tt.wantHourOK)
			}
			if tt.wantMinuteOK && minute.Remaining != tt.wantMinuteRem {
				t.Errorf("minute.Remaining = %d, want %d", minute.Remaining, tt.wantMinuteRem)
			}
			if tt.wantHourOK && hour.Remaining != tt.wantHourRemain {
				t.Errorf("hour.Remaining = %d, want %d", hour.Remaining, tt.wantHourRemain)
			}
		})
	}
}
