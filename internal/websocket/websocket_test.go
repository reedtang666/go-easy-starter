package websocket

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"strings"
	"testing"
	"time"
)

func TestTicketSingleUseAndExpiry(t *testing.T) {
	r := miniredis.RunT(t)
	cl, e := gredis.New(&gredis.Config{Address: r.Addr()})
	if e != nil {
		t.Fatal(e)
	}
	defer cl.Close(context.Background())
	tickets := NewTickets(cl, "test:", time.Second)
	id := contexts.Identity{UserID: 1, SessionID: "s", CredentialVersion: 2}
	v, e := tickets.Issue(context.Background(), id)
	if e != nil {
		t.Fatal(e)
	}
	got, e := tickets.Consume(context.Background(), v)
	if e != nil || got.UserID != 1 {
		t.Fatal(got, e)
	}
	if _, e = tickets.Consume(context.Background(), v); e == nil {
		t.Fatal("ticket reused")
	}
	v, e = tickets.Issue(context.Background(), id)
	if e != nil {
		t.Fatal(e)
	}
	r.FastForward(2 * time.Second)
	if _, e = tickets.Consume(context.Background(), v); e == nil {
		t.Fatal("expired ticket accepted")
	}
	if _, e = tickets.Consume(context.Background(), strings.Repeat("x", 64)); e == nil {
		t.Fatal("invalid ticket accepted")
	}
}
