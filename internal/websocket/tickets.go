package websocket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"time"
)

type Tickets struct {
	redis  *gredis.Redis
	prefix string
	ttl    time.Duration
}

func NewTickets(r *gredis.Redis, prefix string, ttl time.Duration) *Tickets {
	return &Tickets{redis: r, prefix: prefix + "ws-ticket:", ttl: ttl}
}
func (t *Tickets) Issue(ctx context.Context, id contexts.Identity) (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	ticket := hex.EncodeToString(b)
	data, e := json.Marshal(id)
	if e != nil {
		return "", e
	}
	_, e = t.redis.Do(ctx, "SET", t.prefix+ticket, string(data), "EX", int64(t.ttl/time.Second))
	return ticket, e
}
func (t *Tickets) Consume(ctx context.Context, ticket string) (contexts.Identity, error) {
	var id contexts.Identity
	if b, e := hex.DecodeString(ticket); e != nil || len(b) != 32 {
		return id, response.Fail(401, "WS_TICKET_INVALID", "连接票据无效")
	}
	v, e := t.redis.Eval(ctx, `local v=redis.call('GET',KEYS[1]);if v then redis.call('DEL',KEYS[1]) end;return v`, 1, []string{t.prefix + ticket}, nil)
	if e != nil {
		return id, e
	}
	if v.IsEmpty() {
		return id, response.Fail(401, "WS_TICKET_INVALID", "连接票据已失效")
	}
	if e = json.Unmarshal([]byte(v.String()), &id); e != nil {
		return id, e
	}
	return id, nil
}
