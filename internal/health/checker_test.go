package health

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
)

type fakeRedisPinger struct {
	err error
}

func (f fakeRedisPinger) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	if f.err != nil {
		cmd.SetErr(f.err)
	} else {
		cmd.SetVal("PONG")
	}
	return cmd
}

type fakePostgresPinger struct {
	err error
}

func (f fakePostgresPinger) PingContext(ctx context.Context) error {
	return f.err
}

func TestDependencyChecker_Check(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		redisErr   error
		postgres   error
		wantReady  bool
		wantRedis  string
		wantPGStat string
	}{
		{"both healthy", nil, nil, true, "ok", "ok"},
		{"redis unreachable", errors.New("connection refused"), nil, false, "unreachable", "ok"},
		{"postgres unreachable", nil, errors.New("connection refused"), false, "ok", "unreachable"},
		{"both unreachable", errors.New("down"), errors.New("down"), false, "unreachable", "unreachable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checker := NewDependencyChecker(fakeRedisPinger{err: tt.redisErr}, fakePostgresPinger{err: tt.postgres})

			status, ready := checker.Check(context.Background())

			if ready != tt.wantReady {
				t.Errorf("ready = %v, want %v", ready, tt.wantReady)
			}
			if status.Redis != tt.wantRedis {
				t.Errorf("status.Redis = %q, want %q", status.Redis, tt.wantRedis)
			}
			if status.Postgres != tt.wantPGStat {
				t.Errorf("status.Postgres = %q, want %q", status.Postgres, tt.wantPGStat)
			}
		})
	}
}
