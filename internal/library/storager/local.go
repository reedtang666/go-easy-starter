package storager

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"io"
	"os"
	"path"
	"strconv"
	"time"
)

type Local struct {
	root    *os.Root
	cfg     config.Storage
	baseURL string
}

func NewLocal(c config.Storage, base string) (*Local, error) {
	if c.Root == "" || len(c.SigningKey) < 32 {
		return nil, errors.New("local storage requires root and signingKey of at least 32 bytes")
	}
	if e := os.MkdirAll(c.Root, 0700); e != nil {
		return nil, e
	}
	r, e := os.OpenRoot(c.Root)
	if e != nil {
		return nil, e
	}
	return &Local{root: r, cfg: c, baseURL: base}, nil
}
func (l *Local) Close() error { return l.root.Close() }
func (l *Local) Root() string { return l.cfg.Root }
func (l *Local) Put(ctx context.Context, key string, r io.Reader, size int64, mime string) error {
	if e := SafeKey(key); e != nil {
		return e
	}
	if size < 0 {
		return errors.New("invalid file size")
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	if e := l.root.MkdirAll(path.Dir(key), 0700); e != nil {
		return e
	}
	b := make([]byte, 12)
	if _, e := rand.Read(b); e != nil {
		return e
	}
	tmp := key + ".upload-" + hex.EncodeToString(b)
	f, e := l.root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer l.root.Remove(tmp)
	n, e := io.Copy(f, io.LimitReader(&contextReader{ctx: ctx, r: r}, size+1))
	if e != nil {
		f.Close()
		return e
	}
	if n != size {
		f.Close()
		return errors.New("file size does not match declared size")
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return l.root.Rename(tmp, key)
}
func (l *Local) Open(key string) (*os.File, error) {
	if e := SafeKey(key); e != nil {
		return nil, e
	}
	return l.root.Open(key)
}
func (l *Local) Delete(ctx context.Context, key string) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if e := SafeKey(key); e != nil {
		return e
	}
	e := l.root.Remove(key)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	return e
}
func (l *Local) URL(ctx context.Context, key string, private bool, ttl time.Duration) (string, error) {
	if e := SafeKey(key); e != nil {
		return "", e
	}
	if private {
		return "", errors.New("local private URL must be signed by attachment ID")
	}
	return PublicURL(l.baseURL+"/uploads", key)
}
func (l *Local) Sign(id, expires int64) string {
	mac := hmac.New(sha256.New, []byte(l.cfg.SigningKey))
	fmt.Fprintf(mac, "file:%d:%d", id, expires)
	return hex.EncodeToString(mac.Sum(nil))
}
func (l *Local) Verify(id, expires int64, sig string) bool {
	if id <= 0 || expires <= time.Now().Unix() {
		return false
	}
	return hmac.Equal([]byte(l.Sign(id, expires)), []byte(sig))
}
func (l *Local) SignedURL(id int64, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", errors.New("invalid URL lifetime")
	}
	e := time.Now().Add(ttl).Unix()
	return fmt.Sprintf("%s/api/v1/files/%d/content?expires=%s&signature=%s", l.baseURL, id, strconv.FormatInt(e, 10), l.Sign(id, e)), nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(p)
}
