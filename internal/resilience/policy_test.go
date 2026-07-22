package resilience

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestIsRetryableStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status int
		want   bool
	}{
		{502, true},
		{503, true},
		{504, true},
		{200, false},
		{400, false},
		{404, false},
		{500, false},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.status), func(t *testing.T) {
			t.Parallel()
			if got := IsRetryableStatus(tt.status); got != tt.want {
				t.Errorf("IsRetryableStatus(%d) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestRetryPolicy_Backoff(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{BaseBackoff: 100 * time.Millisecond}

	tests := []struct {
		name    string
		attempt int
		maxWant time.Duration
	}{
		{"attempt 1 bounded by base", 1, 100 * time.Millisecond},
		{"attempt 2 bounded by 2x base", 2, 200 * time.Millisecond},
		{"attempt 3 bounded by 4x base", 3, 400 * time.Millisecond},
		{"attempt below 1 treated as 1", 0, 100 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for i := 0; i < 20; i++ {
				got := policy.Backoff(tt.attempt)
				if got < 0 || got >= tt.maxWant {
					t.Fatalf("Backoff(%d) = %v, want in [0, %v)", tt.attempt, got, tt.maxWant)
				}
			}
		})
	}

	t.Run("zero base backoff yields zero delay", func(t *testing.T) {
		t.Parallel()
		zero := RetryPolicy{}
		if got := zero.Backoff(1); got != 0 {
			t.Errorf("Backoff(1) = %v, want 0", got)
		}
	})
}

func TestWithDeadline(t *testing.T) {
	t.Parallel()

	ctx, cancel := WithDeadline(context.Background(), 10*time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Fatal("context should not be done immediately")
	default:
	}

	<-ctx.Done()
	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("ctx.Err() = %v, want %v", ctx.Err(), context.DeadlineExceeded)
	}
}
