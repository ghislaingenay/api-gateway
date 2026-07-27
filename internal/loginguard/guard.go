// Package loginguard tracks consecutive failed login attempts per caller so
// LoginHandler can progressively delay its response, slowing down
// credential-stuffing and brute-force attempts.
package loginguard

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "loginguard:"

// incrScript atomically increments the failure counter and, on its first
// increment, sets its expiry to the configured failure window so the count
// resets once a caller stops failing for that long.
const incrScript = `
local current = redis.call('INCR', KEYS[1])
if current == 1 then
	redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return current
`

// Guard tracks failed login attempts keyed by caller (e.g. IP+tenant+email)
// and resets the count on a successful login.
type Guard interface {
	// RegisterFailure records a failed attempt and returns the number of
	// consecutive failures recorded for key within the current window.
	RegisterFailure(ctx context.Context, key string) (attempt int, err error)
	// Reset clears the failure count for key.
	Reset(ctx context.Context, key string) error
}

// guardStore is the subset of *redis.Client RedisGuard needs.
type guardStore interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

// RedisGuard implements Guard using a Redis counter with a sliding TTL.
type RedisGuard struct {
	redis  guardStore
	window time.Duration
}

// NewRedisGuard returns a Guard backed by Redis, resetting failure counts
// after window has elapsed since the first failure in the current streak.
func NewRedisGuard(redisClient *redis.Client, window time.Duration) *RedisGuard {
	return &RedisGuard{redis: redisClient, window: window}
}

// RegisterFailure implements Guard.
func (g *RedisGuard) RegisterFailure(ctx context.Context, key string) (int, error) {
	res, err := g.redis.Eval(ctx, incrScript, []string{keyPrefix + key}, g.window.Milliseconds()).Result()
	if err != nil {
		return 0, fmt.Errorf("loginguard: eval incr: %w", err)
	}
	count, ok := res.(int64)
	if !ok {
		return 0, fmt.Errorf("loginguard: unexpected eval result: %#v", res)
	}
	return int(count), nil
}

// Reset implements Guard.
func (g *RedisGuard) Reset(ctx context.Context, key string) error {
	if err := g.redis.Del(ctx, keyPrefix+key).Err(); err != nil {
		return fmt.Errorf("loginguard: del: %w", err)
	}
	return nil
}
