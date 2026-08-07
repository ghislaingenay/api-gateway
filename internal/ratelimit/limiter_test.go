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
