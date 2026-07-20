package cache

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type fakeResponseStore struct {
	getResult *redis.StringCmd
	setResult *redis.StatusCmd
	setCalls  []struct {
		key   string
		value interface{}
		ttl   time.Duration
	}
}

func (f *fakeResponseStore) Get(ctx context.Context, key string) *redis.StringCmd {
	return f.getResult
}

func (f *fakeResponseStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) *redis.StatusCmd {
	f.setCalls = append(f.setCalls, struct {
		key   string
		value interface{}
		ttl   time.Duration
	}{key, value, ttl})
	return f.setResult
}

func TestRedisResponseCache_Get(t *testing.T) {
	t.Parallel()

	t.Run("cache miss returns ok=false, no error", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseStore{getResult: redis.NewStringResult("", redis.Nil)}
		c := &redisResponseCache{redis: store}

		resp, hit, err := c.Get(context.Background(), "cache:tenant:GET:/x:hash")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hit || resp != nil {
			t.Fatalf("expected no hit, got hit=%v resp=%v", hit, resp)
		}
	})

	t.Run("redis error returns ok=false and an error, allowing fail-open", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseStore{getResult: redis.NewStringResult("", errors.New("connection refused"))}
		c := &redisResponseCache{redis: store}

		_, hit, err := c.Get(context.Background(), "key")
		if err == nil {
			t.Fatal("expected error")
		}
		if hit {
			t.Fatal("expected no hit on error")
		}
	})

	t.Run("cache hit returns the stored response", func(t *testing.T) {
		t.Parallel()
		want := &CachedResponse{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"ok":true}`)}
		encoded, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("marshal fixture: %v", err)
		}
		store := &fakeResponseStore{getResult: redis.NewStringResult(string(encoded), nil)}
		c := &redisResponseCache{redis: store}

		resp, hit, err := c.Get(context.Background(), "key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !hit {
			t.Fatal("expected hit")
		}
		if resp.StatusCode != want.StatusCode || string(resp.Body) != string(want.Body) {
			t.Fatalf("got %+v, want %+v", resp, want)
		}
	})

	t.Run("corrupt cache entry is treated as a miss", func(t *testing.T) {
		t.Parallel()
		store := &fakeResponseStore{getResult: redis.NewStringResult("not json", nil)}
		c := &redisResponseCache{redis: store}

		resp, hit, err := c.Get(context.Background(), "key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hit || resp != nil {
			t.Fatalf("expected miss for corrupt entry, got hit=%v resp=%v", hit, resp)
		}
	})
}

func TestRedisResponseCache_Set(t *testing.T) {
	t.Parallel()

	store := &fakeResponseStore{setResult: redis.NewStatusResult("OK", nil)}
	c := &redisResponseCache{redis: store}

	resp := &CachedResponse{StatusCode: 200, Header: http.Header{}, Body: []byte("body")}
	if err := c.Set(context.Background(), "key", resp, 60*time.Second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.setCalls) != 1 {
		t.Fatalf("expected 1 set call, got %d", len(store.setCalls))
	}
	if store.setCalls[0].ttl != 60*time.Second {
		t.Fatalf("ttl = %v, want 60s", store.setCalls[0].ttl)
	}
}

func TestBuildKey(t *testing.T) {
	t.Parallel()

	tenantID := "11111111-1111-1111-1111-111111111111"
	got := BuildKey(uuid.MustParse(tenantID), "GET", "/api/orders", "hash123")
	want := "cache:" + tenantID + ":GET:/api/orders:hash123"
	if got != want {
		t.Fatalf("BuildKey() = %q, want %q", got, want)
	}
}
