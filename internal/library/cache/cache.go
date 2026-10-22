package cache

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gogf/gf/v2/database/gredis"
	"time"
)

type Cache struct {
	redis  *gredis.Redis
	prefix string
}

func New(r *gredis.Redis, prefix string) *Cache { return &Cache{redis: r, prefix: prefix} }
func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if ttl < time.Millisecond {
		return errors.New("cache TTL must be at least one millisecond")
	}
	data, e := json.Marshal(value)
	if e != nil {
		return e
	}
	_, e = c.redis.Do(ctx, "SET", c.prefix+key, string(data), "PX", ttl.Milliseconds())
	return e
}
func (c *Cache) Get(ctx context.Context, key string, out any) (bool, error) {
	v, e := c.redis.Do(ctx, "GET", c.prefix+key)
	if e != nil {
		return false, e
	}
	if v.IsNil() {
		return false, nil
	}
	if e = json.Unmarshal([]byte(v.String()), out); e != nil {
		return false, e
	}
	return true, nil
}
func (c *Cache) Remove(ctx context.Context, key string) error {
	_, e := c.redis.Do(ctx, "DEL", c.prefix+key)
	return e
}
