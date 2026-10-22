package storager

import (
	"context"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"strings"
	"testing"
	"time"
)

func TestLocalObjectLifecycle(t *testing.T) {
	d, e := NewLocal(config.Storage{Root: t.TempDir(), SigningKey: strings.Repeat("s", 32)}, "http://localhost:8000")
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	if e = d.Put(ctx, "private/a.txt", strings.NewReader("hello"), 5, "text/plain"); e != nil {
		t.Fatal(e)
	}
	r, e := d.Open("private/a.txt")
	if e != nil {
		t.Fatal(e)
	}
	r.Close()
	if _, e = d.URL(ctx, "private/a.txt", false, time.Minute); e != nil {
		t.Fatal(e)
	}
	if e = d.Delete(ctx, "private/a.txt"); e != nil {
		t.Fatal(e)
	}
	if e = d.Delete(ctx, "private/a.txt"); e != nil {
		t.Fatal("delete should be idempotent", e)
	}
}
func TestLocalRejectsTraversal(t *testing.T) {
	d, _ := NewLocal(config.Storage{Root: t.TempDir(), SigningKey: strings.Repeat("s", 32)}, "http://localhost:8000")
	for _, key := range []string{"../escape", "/tmp/escape", "a/../../escape", "a\\..\\escape", ""} {
		if e := d.Put(context.Background(), key, strings.NewReader("x"), 1, "text/plain"); e == nil {
			t.Fatalf("accepted unsafe key %q", key)
		}
	}
}
func TestLocalSignature(t *testing.T) {
	d, _ := NewLocal(config.Storage{Root: t.TempDir(), SigningKey: strings.Repeat("s", 32)}, "http://localhost:8000")
	exp := time.Now().Add(time.Minute).Unix()
	sig := d.Sign(14, exp)
	if !d.Verify(14, exp, sig) || d.Verify(15, exp, sig) || d.Verify(14, time.Now().Unix()-1, sig) {
		t.Fatal("invalid signature behavior")
	}
}
func TestAllDriverFactoriesAndCapabilities(t *testing.T) {
	for _, name := range []string{"local", "oss", "cos", "qiniu", "minio", "ucloud"} {
		t.Run(name, func(t *testing.T) {
			cfg := config.Storage{Driver: name, Root: t.TempDir(), SigningKey: strings.Repeat("s", 32), Endpoint: "http://localhost:9000", PublicURL: "https://files.example.test", Bucket: "starter", AccessKey: "test", SecretKey: "test", Secure: false}
			if name == "minio" {
				cfg.Endpoint = "localhost:9000"
			}
			d, e := New(cfg, "http://localhost:8000")
			if e != nil {
				t.Fatal(e)
			}
			if d == nil {
				t.Fatal("missing driver")
			}
		})
	}
}
