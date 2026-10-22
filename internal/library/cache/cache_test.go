package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
	"github.com/gogf/gf/v2/database/gredis"
)

func TestJSONCacheLifecycle(t *testing.T) {
	r := miniredis.RunT(t)
	redis, err := gredis.New(&gredis.Config{Address: r.Addr()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	defer redis.Close(ctx)
	c := New(redis, "test:")
	var got struct{ Name string }
	if found, err := c.Get(ctx, "missing", &got); err != nil || found {
		t.Fatalf("missing: %v %v", found, err)
	}
	if err := c.Set(ctx, "user", map[string]string{"Name": "starter"}, time.Second); err != nil {
		t.Fatal(err)
	}
	if found, err := c.Get(ctx, "user", &got); err != nil || !found || got.Name != "starter" {
		t.Fatalf("get: %+v %v %v", got, found, err)
	}
	if !r.Exists("test:user") {
		t.Fatal("key prefix missing")
	}
	r.FastForward(2 * time.Second)
	if found, err := c.Get(ctx, "user", &got); err != nil || found {
		t.Fatalf("expiry: %v %v", found, err)
	}
	if err := c.Set(ctx, "user", 1, time.Second); err != nil {
		t.Fatal(err)
	}
	if err := c.Remove(ctx, "user"); err != nil {
		t.Fatal(err)
	}
	if r.Exists("test:user") {
		t.Fatal("remove failed")
	}
	if err := c.Set(ctx, "user", 1, 0); err == nil {
		t.Fatal("zero TTL accepted")
	}
}
