package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

type item struct {
	ID   int
	Name string
}

func newTestCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	return New(Config{Addr: server.Addr(), TTL: time.Minute}, nil), server
}

func TestSetGetRoundTrip(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()

	var got []item
	if c.Get(ctx, "test", "h", "f", &got) {
		t.Fatal("expected a miss on an empty cache")
	}

	c.Set(ctx, "h", "f", []item{{1, "a"}, {2, "b"}})
	if !c.Get(ctx, "test", "h", "f", &got) || len(got) != 2 || got[1].Name != "b" {
		t.Fatalf("unexpected value: %+v", got)
	}
}

func TestInvalidateRemovesEveryField(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	c.Set(ctx, "h", "1", item{1, "a"})
	c.Set(ctx, "h", "2", item{2, "b"})

	c.Invalidate(ctx, "h")

	var got item
	if c.Get(ctx, "test", "h", "1", &got) || c.Get(ctx, "test", "h", "2", &got) {
		t.Fatal("all fields of the hash should be gone")
	}
}

func TestTTLIsSetOnFirstWriteOnly(t *testing.T) {
	c, server := newTestCache(t)
	ctx := context.Background()
	c.Set(ctx, "h", "1", item{1, "a"})
	server.FastForward(40 * time.Second)
	c.Set(ctx, "h", "2", item{2, "b"}) // 2回目の書き込みでは、TTL は延びない
	server.FastForward(30 * time.Second)

	var got item
	if c.Get(ctx, "test", "h", "1", &got) {
		t.Fatal("the hash should have expired 60s after the first write")
	}
}

func TestNilCacheIsANoOp(t *testing.T) {
	var c *Cache // REDIS_ENABLED=false のときの状態
	ctx := context.Background()

	var got item
	if c.Get(ctx, "test", "h", "f", &got) {
		t.Fatal("a nil cache must always miss")
	}
	c.Set(ctx, "h", "f", item{1, "a"})
	c.Invalidate(ctx, "h")
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRedisDownFallsBackToMiss(t *testing.T) {
	c, server := newTestCache(t)
	server.Close()
	ctx := context.Background()

	var got item
	if c.Get(ctx, "test", "h", "f", &got) {
		t.Fatal("must be a miss, not a hit, when redis is unreachable")
	}
	c.Set(ctx, "h", "f", item{1, "a"}) // パニックやブロックをしない
	c.Invalidate(ctx, "h")
}
