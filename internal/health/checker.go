// Package health checks connectivity to the gateway's runtime dependencies
// (Redis, PostgreSQL) for the readiness probe (FEAT-009).
package health

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// checkTimeout bounds each dependency check so /ready always responds
// promptly even if a dependency is unresponsive rather than erroring
// (FEAT-009 Risk 1).
const checkTimeout = 2 * time.Second

// RedisPinger is the subset of *redis.Client needed to check connectivity.
// Declared here (the consumer) per the DI convention; *redis.Client
// satisfies it structurally.
type RedisPinger interface {
	Ping(ctx context.Context) *redis.StatusCmd
}

// PostgresPinger is the subset of *sql.DB needed to check connectivity.
// Declared here (the consumer) per the DI convention; *sql.DB satisfies it
// structurally.
type PostgresPinger interface {
	PingContext(ctx context.Context) error
}

// Status reports each dependency's connectivity as "ok" or "unreachable".
type Status struct {
	Redis    string
	Postgres string
}

// DependencyChecker checks Redis and PostgreSQL connectivity for the
// readiness probe (FEAT-009 FR-3).
type DependencyChecker struct {
	redis    RedisPinger
	postgres PostgresPinger
}

// NewDependencyChecker returns a DependencyChecker backed by the given
// Redis and PostgreSQL connections.
func NewDependencyChecker(redis RedisPinger, postgres PostgresPinger) *DependencyChecker {
	return &DependencyChecker{redis: redis, postgres: postgres}
}

// Check pings Redis and PostgreSQL, each bounded by checkTimeout, and
// reports per-dependency status. ready is true only if both succeed.
func (c *DependencyChecker) Check(ctx context.Context) (status Status, ready bool) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	status.Redis = "ok"
	if err := c.redis.Ping(ctx).Err(); err != nil {
		status.Redis = "unreachable"
	}

	status.Postgres = "ok"
	if err := c.postgres.PingContext(ctx); err != nil {
		status.Postgres = "unreachable"
	}

	ready = status.Redis == "ok" && status.Postgres == "ok"
	return status, ready
}
