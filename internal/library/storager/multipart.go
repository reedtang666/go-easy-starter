package storager

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofrs/flock"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

type Manifest struct {
	ID          string `json:"uploadId"`
	Owner       int64  `json:"-"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ChunkSize   int64  `json:"chunkSize"`
	Count       int    `json:"count"`
	Digest      string `json:"digest"`
	Private     bool   `json:"private"`
	Expires     int64  `json:"expiresAt"`
	Parts       []int  `json:"parts"`
	CompletedID int64  `json:"attachmentId,omitempty"`
}
type diskManifest struct {
	Manifest
	OwnerID int64 `json:"ownerId"`
}
type Multipart struct {
	local  *Local
	max    int64
	ttl    time.Duration
	finish func(context.Context, Manifest, io.ReadSeeker) (int64, error)
}

var multipartID = regexp.MustCompile(`^[a-f0-9]{40}$`)

func NewMultipart(l *Local, max int64, ttl time.Duration, finish func(context.Context, Manifest, io.ReadSeeker) (int64, error)) *Multipart {
	return &Multipart{local: l, max: max, ttl: ttl, finish: finish}
}
func (m *Multipart) Init(owner int64, name string, size, chunk int64, digest string, private bool) (*Manifest, error) {
	if owner <= 0 || size <= 0 || size > m.max || chunk < 65536 || chunk > 4<<20 {
		return nil, response.Fail(400, "MULTIPART_INVALID", "分片大小或文件大小无效")
	}
	b, e := hex.DecodeString(digest)
	if e != nil || len(b) != 32 {
		return nil, response.Fail(400, "MULTIPART_INVALID", "需提供完整文件SHA256")
	}
	b = make([]byte, 20)
	if _, e = rand.Read(b); e != nil {
		return nil, e
	}
	v := &Manifest{ID: hex.EncodeToString(b), Owner: owner, Name: filepath.Base(name), Size: size, ChunkSize: chunk, Count: int((size + chunk - 1) / chunk), Digest: digest, Private: private, Expires: time.Now().Add(m.ttl).Unix(), Parts: []int{}}
	if e = m.save(v); e != nil {
		return nil, e
	}
	return v, nil
}
func (m *Multipart) load(id string) (*Manifest, error) {
	if !multipartID.MatchString(id) {
		return nil, response.Fail(404, "UPLOAD_NOT_FOUND", "上传会话不存在")
	}
	f, e := m.local.Open(".multipart/" + id + "/manifest.json")
	if e != nil {
		if errors.Is(e, os.ErrNotExist) {
			return nil, response.Fail(404, "UPLOAD_NOT_FOUND", "上传会话不存在")
		}
		return nil, e
	}
	defer f.Close()
	var d diskManifest
	if e = json.NewDecoder(f).Decode(&d); e != nil {
		return nil, e
	}
	d.Manifest.Owner = d.OwnerID
	return &d.Manifest, nil
}
func (m *Multipart) save(v *Manifest) error {
	b, e := json.Marshal(diskManifest{Manifest: *v, OwnerID: v.Owner})
	if e != nil {
		return e
	}
	return m.local.Put(context.Background(), ".multipart/"+v.ID+"/manifest.json", bytes.NewReader(b), int64(len(b)), "application/json")
}
func (m *Multipart) locked(ctx context.Context, owner int64, id string, fn func(*Manifest) error) error {
	if _, e := m.load(id); e != nil {
		return e
	}
	if e := m.local.root.MkdirAll(".multipart/locks", 0700); e != nil {
		return e
	}
	lock := flock.New(filepath.Join(m.local.cfg.Root, ".multipart", "locks", id+".lock"))
	ok, e := lock.TryLockContext(ctx, 20*time.Millisecond)
	if e != nil {
		return e
	}
	if !ok {
		return response.Fail(409, "UPLOAD_BUSY", "上传会话正在处理")
	}
	defer lock.Close()
	v, e := m.load(id)
	if e != nil {
		return e
	}
	if v.Owner != owner {
		return response.Fail(404, "UPLOAD_NOT_FOUND", "上传会话不存在")
	}
	if time.Now().Unix() >= v.Expires {
		return response.Fail(410, "UPLOAD_EXPIRED", "上传会话已过期")
	}
	return fn(v)
}
func (m *Multipart) Get(ctx context.Context, owner int64, id string) (*Manifest, error) {
	var result *Manifest
	e := m.locked(ctx, owner, id, func(v *Manifest) error { result = v; return nil })
	return result, e
}
func (m *Multipart) Part(ctx context.Context, owner int64, id string, index int, r io.Reader, size int64) error {
	return m.locked(ctx, owner, id, func(v *Manifest) error {
		if v.CompletedID > 0 {
			return response.Fail(409, "UPLOAD_COMPLETE", "上传会话已完成")
		}
		if index < 0 || index >= v.Count {
			return response.Fail(400, "PART_INVALID", "分片编号无效")
		}
		expected := v.ChunkSize
		if index == v.Count-1 {
			expected = v.Size - int64(index)*v.ChunkSize
		}
		if size != expected {
			return response.Fail(400, "PART_SIZE_INVALID", "分片大小无效")
		}
		key := fmt.Sprintf(".multipart/%s/parts/%08d", id, index)
		if e := m.local.Put(ctx, key, r, size, "application/octet-stream"); e != nil {
			return e
		}
		found := false
		for _, n := range v.Parts {
			if n == index {
				found = true
			}
		}
		if !found {
			v.Parts = append(v.Parts, index)
			sort.Ints(v.Parts)
		}
		return m.save(v)
	})
}
func (m *Multipart) Complete(ctx context.Context, owner int64, id string) (int64, error) {
	var result int64
	e := m.locked(ctx, owner, id, func(v *Manifest) error {
		if v.CompletedID > 0 {
			result = v.CompletedID
			return nil
		}
		if len(v.Parts) != v.Count {
			return response.Fail(409, "PARTS_MISSING", "分片尚未全部上传")
		}
		if m.finish == nil {
			return errors.New("multipart completion handler missing")
		}
		files := []*os.File{}
		defer func() {
			for _, f := range files {
				f.Close()
			}
		}()
		readers := []io.Reader{}
		for i := 0; i < v.Count; i++ {
			f, e := m.local.Open(fmt.Sprintf(".multipart/%s/parts/%08d", id, i))
			if e != nil {
				return e
			}
			files = append(files, f)
			readers = append(readers, f)
		}
		key := ".multipart/" + id + "/assembled"
		if e := m.local.Put(ctx, key, io.MultiReader(readers...), v.Size, "application/octet-stream"); e != nil {
			return e
		}
		assembled, e := m.local.Open(key)
		if e != nil {
			return e
		}
		defer assembled.Close()
		hash := sha256.New()
		if _, e = io.Copy(hash, assembled); e != nil {
			return e
		}
		if hex.EncodeToString(hash.Sum(nil)) != v.Digest {
			return response.Fail(400, "FILE_DIGEST_MISMATCH", "完整文件校验失败")
		}
		if _, e = assembled.Seek(0, io.SeekStart); e != nil {
			return e
		}
		result, e = m.finish(ctx, *v, assembled)
		if e != nil {
			return e
		}
		v.CompletedID = result
		return m.save(v)
	})
	return result, e
}
func (m *Multipart) Cleanup(ctx context.Context) error {
	entries, e := os.ReadDir(filepath.Join(m.local.cfg.Root, ".multipart"))
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if !entry.IsDir() || !multipartID.MatchString(entry.Name()) {
			continue
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		v, e := m.load(entry.Name())
		if e != nil {
			return e
		}
		if v.Expires > time.Now().Unix() {
			continue
		}
		lock := flock.New(filepath.Join(m.local.cfg.Root, ".multipart", "locks", v.ID+".lock"))
		if e = m.local.root.MkdirAll(".multipart/locks", 0700); e != nil {
			return e
		}
		ok, e := lock.TryLock()
		if e != nil {
			return e
		}
		if !ok {
			lock.Close()
			continue
		}
		e = m.local.root.RemoveAll(".multipart/" + v.ID)
		lock.Close()
		if e != nil {
			return e
		}
		if e = os.Remove(filepath.Join(m.local.cfg.Root, ".multipart", "locks", v.ID+".lock")); e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
	}
	return nil
}

func CleanupLocal(ctx context.Context, root string, max int64, ttl time.Duration) error {
	r, e := os.OpenRoot(root)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	defer r.Close()
	local := &Local{root: r}
	local.cfg.Root = root
	return NewMultipart(local, max, ttl, nil).Cleanup(ctx)
}
