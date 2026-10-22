package auth

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"strings"
	"sync"
	"testing"
	"time"
)

func testManager(t *testing.T) (*Manager, *miniredis.Miniredis, *time.Time) {
	t.Helper()
	r := miniredis.RunT(t)
	client, err := gredis.New(&gredis.Config{Address: r.Addr()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close(context.Background()) })
	store, err := NewRedisStore(client, "test:auth:")
	if err != nil {
		t.Fatal(err)
	}
	c := config.Defaults().Auth
	c.AccessSecretKey = strings.Repeat("a", 32)
	c.RefreshHashKey = strings.Repeat("b", 32)
	now := time.Now()
	m, err := NewManager(c, store, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return m, r, &now
}
func TestLoginReplacementAndLogout(t *testing.T) {
	m, _, _ := testManager(t)
	ctx := context.Background()
	p, _, e := m.Login(ctx, 1, 1, Device{ID: "one"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.ValidateAccess(ctx, p.AccessToken); e != nil {
		t.Fatal(e)
	}
	p2, _, e := m.Login(ctx, 1, 1, Device{ID: "two"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.ValidateAccess(ctx, p.AccessToken); e == nil {
		t.Fatal("old login still valid")
	}
	session, e := m.ValidateAccess(ctx, p2.AccessToken)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Logout(ctx, 1, session.SessionID); e != nil {
		t.Fatal(e)
	}
	if _, e = m.ValidateAccess(ctx, p2.AccessToken); e == nil {
		t.Fatal("logout not enforced")
	}
}
func TestRefreshConcurrentReplayAndRevocation(t *testing.T) {
	m, r, now := testManager(t)
	ctx := context.Background()
	p, _, e := m.Login(ctx, 4, 7, Device{ID: "d"})
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	out := make(chan *TokenPair, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := m.Refresh(ctx, p.RefreshToken, "d")
			if e != nil {
				errs <- e
				return
			}
			out <- v
		}()
	}
	wg.Wait()
	close(out)
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	var first string
	for v := range out {
		if first == "" {
			first = v.RefreshToken
		}
		if first != v.RefreshToken {
			t.Fatal("non-idempotent refresh")
		}
	}
	if _, e = m.Refresh(ctx, strings.Split(p.RefreshToken, ".")[0]+".invalid", "d"); e == nil {
		t.Fatal("invalid token accepted")
	}
	if _, e = m.ValidateAccess(ctx, p.AccessToken); e != nil {
		t.Fatal("invalid input revoked real session", e)
	}
	r.FastForward(6 * time.Second)
	*now = now.Add(6 * time.Second)
	if _, e = m.Refresh(ctx, p.RefreshToken, "d"); e == nil {
		t.Fatal("spent token accepted")
	}
	if _, e = m.ValidateAccess(ctx, p.AccessToken); e == nil {
		t.Fatal("legitimate replay must revoke session")
	}
}
func TestTokenExpiryAndCredentialClaims(t *testing.T) {
	m, _, now := testManager(t)
	p, _, e := m.Login(context.Background(), 1, 3, Device{ID: "d"})
	if e != nil {
		t.Fatal(e)
	}
	s, c, e := m.ValidateAccessWithClaims(context.Background(), p.AccessToken)
	if e != nil || c.CredentialVersion != 3 || s.CredentialVersion != 3 {
		t.Fatalf("claims mismatch %v", e)
	}
	*now = now.Add(16 * time.Minute)
	if _, e = m.ValidateAccess(context.Background(), p.AccessToken); e == nil {
		t.Fatal("expired token accepted")
	}
}
