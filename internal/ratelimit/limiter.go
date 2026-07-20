package ratelimit

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Window identifies a rate-limit window granularity.
type Window string

const (
	WindowMinute Window = "minute"
	WindowHour   Window = "hour"
)

// Duration returns the wall-clock length of the window.
func (w Window) Duration() time.Duration {
	switch w {
	case WindowMinute:
		return time.Minute
	case WindowHour:
		return time.Hour
	default:
		return 0
	}
}

const keyPrefix = "ratelimit:"

// incrScript atomically increments the current bucket (initializing its
// expiry on first write) and reads the previous bucket's count, so the two
// reads used by the sliding-window approximation never race against a
// concurrent request for the same tenant/user.
const incrScript = `
local current = redis.call('INCR', KEYS[1])
if current == 1 then
	redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
local previous = redis.call('GET', KEYS[2])
if previous == false then
	previous = '0'
end
return {current, previous}
`

// Decision is the outcome of a rate-limit check for one window.
type Decision struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
}

// Limiter enforces a per-tenant, per-user rate limit for a single window.
type Limiter interface {
	Allow(ctx context.Context, tenantID, userID uuid.UUID, window Window, limit int) (Decision, error)
}

// limiterStore is the subset of *redis.Client the limiter needs, sized to
// its one call so tests can substitute a fake without a live Redis instance.
type limiterStore interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
}

// SlidingWindowLimiter implements Limiter using the Cloudflare-style
// two-bucket sliding-window approximation: the current fixed-size bucket
// count is combined with a weighted fraction of the previous bucket's
// count, based on how far into the current bucket the request falls.
type SlidingWindowLimiter struct {
	redis limiterStore
	now   func() time.Time
}

// NewSlidingWindowLimiter returns a Limiter backed by Redis.
func NewSlidingWindowLimiter(redisClient *redis.Client) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{redis: redisClient, now: time.Now}
}

// Allow implements Limiter.
func (l *SlidingWindowLimiter) Allow(ctx context.Context, tenantID, userID uuid.UUID, window Window, limit int) (Decision, error) {
	dur := window.Duration()
	if dur <= 0 {
		return Decision{}, fmt.Errorf("ratelimit: unknown window %q", window)
	}

	now := l.now().UTC()
	bucketStart := now.Truncate(dur)
	prevBucketStart := bucketStart.Add(-dur)

	currentKey := fmt.Sprintf("%s%s:%s:%s:%d", keyPrefix, tenantID, userID, window, bucketStart.Unix())
	previousKey := fmt.Sprintf("%s%s:%s:%s:%d", keyPrefix, tenantID, userID, window, prevBucketStart.Unix())

	res, err := l.redis.Eval(ctx, incrScript, []string{currentKey, previousKey}, (2 * dur).Milliseconds()).Result()
	if err != nil {
		return Decision{}, fmt.Errorf("ratelimit: eval sliding window: %w", err)
	}

	current, previous, err := parseCounts(res)
	if err != nil {
		return Decision{}, fmt.Errorf("ratelimit: parse sliding window result: %w", err)
	}

	elapsed := now.Sub(bucketStart)
	weight := 1 - float64(elapsed)/float64(dur)
	if weight < 0 {
		weight = 0
	}
	weightedCount := float64(current) + float64(previous)*weight

	remaining := limit - int(math.Ceil(weightedCount))
	if remaining < 0 {
		remaining = 0
	}

	return Decision{
		Allowed:    weightedCount <= float64(limit),
		Limit:      limit,
		Remaining:  remaining,
		RetryAfter: dur - elapsed,
	}, nil
}

func parseCounts(res interface{}) (current, previous int64, err error) {
	values, ok := res.([]interface{})
	if !ok || len(values) != 2 {
		return 0, 0, fmt.Errorf("unexpected eval result shape: %#v", res)
	}

	current, ok = toInt64(values[0])
	if !ok {
		return 0, 0, fmt.Errorf("unexpected current bucket value: %#v", values[0])
	}
	previous, ok = toInt64(values[1])
	if !ok {
		return 0, 0, fmt.Errorf("unexpected previous bucket value: %#v", values[1])
	}

	return current, previous, nil
}

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case string:
		parsed, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
