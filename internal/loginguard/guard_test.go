package loginguard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// fakeGuardStore is an in-memory stand-in for the Redis subset RedisGuard
// needs, so tests don't require a live Redis instance.
type fakeGuardStore struct {
	counts    map[string]int64
	evalErr   error
	delErr    error
	delCalls  []string
	evalCalls []string
}

func newFakeGuardStore() *fakeGuardStore {
	return &fakeGuardStore{counts: map[string]int64{}}
}

func (f *fakeGuardStore) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	cmd := redis.NewCmd(ctx)
	f.evalCalls = append(f.evalCalls, keys[0])
	if f.evalErr != nil {
		cmd.SetErr(f.evalErr)
		return cmd
	}
	f.counts[keys[0]]++
	cmd.SetVal(f.counts[keys[0]])
	return cmd
}

func (f *fakeGuardStore) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	f.delCalls = append(f.delCalls, keys...)
	if f.delErr != nil {
		cmd.SetErr(f.delErr)
		return cmd
	}
	for _, k := range keys {
		delete(f.counts, k)
	}
	cmd.SetVal(int64(len(keys)))
	return cmd
}

func TestRedisGuard_RegisterFailure(t *testing.T) {
	t.Parallel()

	store := newFakeGuardStore()
	guard := &RedisGuard{redis: store, window: time.Minute}

	for i, want := range []int{1, 2, 3} {
		got, err := guard.RegisterFailure(context.Background(), "203.0.113.7:acme:user@acme.test")
		if err != nil {
			t.Fatalf("attempt %d: RegisterFailure() error = %v", i, err)
		}
		if got != want {
			t.Errorf("attempt %d: RegisterFailure() = %d, want %d", i, got, want)
		}
	}

	if len(store.evalCalls) != 3 || store.evalCalls[0] != keyPrefix+"203.0.113.7:acme:user@acme.test" {
		t.Errorf("evalCalls = %v, want 3 calls against the prefixed key", store.evalCalls)
	}
}

func TestRedisGuard_RegisterFailure_DifferentKeysAreIndependent(t *testing.T) {
	t.Parallel()

	store := newFakeGuardStore()
	guard := &RedisGuard{redis: store, window: time.Minute}

	if _, err := guard.RegisterFailure(context.Background(), "a"); err != nil {
		t.Fatalf("RegisterFailure(a) error = %v", err)
	}
	got, err := guard.RegisterFailure(context.Background(), "b")
	if err != nil {
		t.Fatalf("RegisterFailure(b) error = %v", err)
	}
	if got != 1 {
		t.Errorf("RegisterFailure(b) = %d, want 1 (independent from key a)", got)
	}
}

func TestRedisGuard_RegisterFailure_RedisError(t *testing.T) {
	t.Parallel()

	store := newFakeGuardStore()
	store.evalErr = errors.New("connection refused")
	guard := &RedisGuard{redis: store, window: time.Minute}

	if _, err := guard.RegisterFailure(context.Background(), "key"); err == nil {
		t.Fatal("expected an error when Redis is unavailable")
	}
}

func TestRedisGuard_Reset(t *testing.T) {
	t.Parallel()

	store := newFakeGuardStore()
	guard := &RedisGuard{redis: store, window: time.Minute}

	key := "203.0.113.7:acme:user@acme.test"
	if _, err := guard.RegisterFailure(context.Background(), key); err != nil {
		t.Fatalf("RegisterFailure() error = %v", err)
	}

	if err := guard.Reset(context.Background(), key); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	got, err := guard.RegisterFailure(context.Background(), key)
	if err != nil {
		t.Fatalf("RegisterFailure() after reset error = %v", err)
	}
	if got != 1 {
		t.Errorf("RegisterFailure() after reset = %d, want 1", got)
	}
}

func TestRedisGuard_Reset_RedisError(t *testing.T) {
	t.Parallel()

	store := newFakeGuardStore()
	store.delErr = errors.New("connection refused")
	guard := &RedisGuard{redis: store, window: time.Minute}

	if err := guard.Reset(context.Background(), "key"); err == nil {
		t.Fatal("expected an error when Redis is unavailable")
	}
}
