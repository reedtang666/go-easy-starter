package storager

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMultipartOutOfOrderAndConcurrentCompletion(t *testing.T) {
	l, e := NewLocal(config.Storage{Root: t.TempDir(), SigningKey: strings.Repeat("x", 32)}, "http://localhost")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	content := bytes.Repeat([]byte("a"), 65537)
	sum := sha256.Sum256(content)
	var calls atomic.Int32
	m := NewMultipart(l, 1<<20, time.Hour, func(_ context.Context, v Manifest, r io.ReadSeeker) (int64, error) {
		b, e := io.ReadAll(r)
		if !bytes.Equal(b, content) {
			t.Error("incorrect assembly")
		}
		calls.Add(1)
		return 42, e
	})
	v, e := m.Init(1, "test.txt", int64(len(content)), 65536, hex.EncodeToString(sum[:]), true)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e = m.Part(ctx, 2, v.ID, 1, bytes.NewReader(content[65536:]), 1); e == nil {
		t.Fatal("wrong owner accepted")
	}
	if e = m.Part(ctx, 1, v.ID, 1, bytes.NewReader(content[65536:]), 1); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Complete(ctx, 1, v.ID); e == nil {
		t.Fatal("missing chunk accepted")
	}
	if e = m.Part(ctx, 1, v.ID, 0, bytes.NewReader(content[:65536]), 65536); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, e := m.Complete(ctx, 1, v.ID)
			if e != nil || id != 42 {
				t.Error(id, e)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicate completion", calls.Load())
	}
}
