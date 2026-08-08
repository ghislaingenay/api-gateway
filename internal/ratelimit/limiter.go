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
	WindowDay    Window = "day"
)

// Duration returns the wall-clock length of the window.
func (w Window) Duration() time.Duration {
	switch w {
	case WindowMinute:
		return time.Minute
	case WindowHour:
		return time.Hour
	case WindowDay:
		return 24 * time.Hour
	default:
		return 0
	}
}

const keyPrefix = "ratelimit:"

// incrFunc atomically increments the current bucket (initializing its
// expiry on first write) and reads the previous bucket's count, so the two
// reads used by the sliding-window approximation never race against a
// concurrent request for the same tenant/user. Shared by incrScript and
// bothWindowsScript via string concatenation so the two never drift apart.
const incrFunc = `
local function incr(curKey, prevKey, ttl)
	local current = redis.call('INCR', curKey)
	if current == 1 then
		redis.call('PEXPIRE', curKey, ttl)
	end
	local previous = redis.call('GET', prevKey)
	if previous == false then
		previous = '0'
	end
	return {current, previous}
end
`

// bothWindowsScript checks the minute window (KEYS[1..2], ARGV[1]) and the
// hour window (KEYS[3..4], ARGV[2]) in one round trip, so a combined check
// against both windows costs a single Redis call instead of two.
const bothWindowsScript = incrFunc + `
local minute = incr(KEYS[1], KEYS[2], ARGV[1])
local hour = incr(KEYS[3], KEYS[4], ARGV[2])
return {minute[1], minute[2], hour[1], hour[2]}
`

// singleWindowScript checks one arbitrary-duration window (KEYS[1..2],
// ARGV[1]), for limits with no natural minute/hour pairing — e.g. a
// per-day cap on a single expensive route rather than the generic
// per-tenant/user API limit.
const singleWindowScript = incrFunc + `
return incr(KEYS[1], KEYS[2], ARGV[1])
`

// Decision is the outcome of a rate-limit check for one window.
type Decision struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Duration
}

// MultiWindowLimiter enforces a per-tenant, per-user rate limit across both
// the minute and hour windows in a single call, so an implementation can
// satisfy both checks with one round trip to its backing store instead of
// two sequential ones.
type MultiWindowLimiter interface {
	AllowBoth(ctx context.Context, tenantID, userID uuid.UUID, minuteLimit, hourLimit int) (minute, hour Decision, err error)
}

// SingleWindowLimiter enforces a limit over one arbitrary window against a
// caller-chosen key, for route-specific limits (e.g. onboarding's
// one-per-day cap) that don't fit the generic tenant/user minute+hour
// shape MultiWindowLimiter provides.
type SingleWindowLimiter interface {
	Allow(ctx context.Context, key string, window Window, limit int) (Decision, error)
}

// Limiter is satisfied by an implementation providing both the generic
// per-tenant/user API rate limit and the single-window limit used for
// stricter, route-specific limits.
type Limiter interface {
	MultiWindowLimiter
	SingleWindowLimiter
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

// AllowBoth implements MultiWindowLimiter, checking the minute and hour
// windows for tenantID/userID with a single Redis round trip
// (bothWindowsScript) instead of two sequential per-window evals.
func (l *SlidingWindowLimiter) AllowBoth(ctx context.Context, tenantID, userID uuid.UUID, minuteLimit, hourLimit int) (Decision, Decision, error) {
	key := fmt.Sprintf("%s:%s", tenantID, userID)

	now := l.now().UTC()
	minuteBucket := now.Truncate(WindowMinute.Duration())
	hourBucket := now.Truncate(WindowHour.Duration())

	keys := []string{
		bucketKey(key, WindowMinute, minuteBucket),
		bucketKey(key, WindowMinute, minuteBucket.Add(-WindowMinute.Duration())),
		bucketKey(key, WindowHour, hourBucket),
		bucketKey(key, WindowHour, hourBucket.Add(-WindowHour.Duration())),
	}

	res, err := l.redis.Eval(ctx, bothWindowsScript, keys,
		(2 * WindowMinute.Duration()).Milliseconds(), (2 * WindowHour.Duration()).Milliseconds(),
	).Result()
	if err != nil {
		return Decision{}, Decision{}, fmt.Errorf("ratelimit: eval both windows: %w", err)
	}

	values, ok := res.([]interface{})
	if !ok || len(values) != 4 {
		return Decision{}, Decision{}, fmt.Errorf("ratelimit: unexpected eval result shape: %#v", res)
	}

	minuteCurrent, minutePrevious, err := toCounts(values[0], values[1])
	if err != nil {
		return Decision{}, Decision{}, fmt.Errorf("ratelimit: parse minute window result: %w", err)
	}
	hourCurrent, hourPrevious, err := toCounts(values[2], values[3])
	if err != nil {
		return Decision{}, Decision{}, fmt.Errorf("ratelimit: parse hour window result: %w", err)
	}

	minuteDecision := buildDecision(minuteCurrent, minutePrevious, now, minuteBucket, WindowMinute.Duration(), minuteLimit)
	hourDecision := buildDecision(hourCurrent, hourPrevious, now, hourBucket, WindowHour.Duration(), hourLimit)

	return minuteDecision, hourDecision, nil
}

// Allow implements SingleWindowLimiter, checking key against one window
// with a single Redis round trip (singleWindowScript).
func (l *SlidingWindowLimiter) Allow(ctx context.Context, key string, window Window, limit int) (Decision, error) {
	now := l.now().UTC()
	bucket := now.Truncate(window.Duration())

	keys := []string{
		bucketKey(key, window, bucket),
		bucketKey(key, window, bucket.Add(-window.Duration())),
	}

	res, err := l.redis.Eval(ctx, singleWindowScript, keys, (2 * window.Duration()).Milliseconds()).Result()
	if err != nil {
		return Decision{}, fmt.Errorf("ratelimit: eval single window: %w", err)
	}

	values, ok := res.([]interface{})
	if !ok || len(values) != 2 {
		return Decision{}, fmt.Errorf("ratelimit: unexpected eval result shape: %#v", res)
	}

	current, previous, err := toCounts(values[0], values[1])
	if err != nil {
		return Decision{}, fmt.Errorf("ratelimit: parse window result: %w", err)
	}

	return buildDecision(current, previous, now, bucket, window.Duration(), limit), nil
}

func bucketKey(key string, window Window, bucketStart time.Time) string {
	return fmt.Sprintf("%s%s:%s:%d", keyPrefix, key, window, bucketStart.Unix())
}

// buildDecision turns a bucket's current/previous counts into a Decision,
// weighting the previous bucket by how much of the current one has already
// elapsed (the sliding-window approximation).
func buildDecision(current, previous int64, now, bucketStart time.Time, dur time.Duration, limit int) Decision {
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
	}
}

func toCounts(curRaw, prevRaw interface{}) (current, previous int64, err error) {
	current, ok := toInt64(curRaw)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected current bucket value: %#v", curRaw)
	}
	previous, ok = toInt64(prevRaw)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected previous bucket value: %#v", prevRaw)
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
