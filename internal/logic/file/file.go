package file

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"github.com/reedtang666/go-easy-starter/internal/dao"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"github.com/reedtang666/go-easy-starter/internal/library/storager"
	"github.com/reedtang666/go-easy-starter/internal/model"
	view "github.com/reedtang666/go-easy-starter/internal/model/view"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type sFile struct {
	store     *dao.Store
	cfg       config.Config
	drivers   map[string]storager.Driver
	Multipart *storager.Multipart
}

func New(s *dao.Store, c config.Config) (*sFile, error) {
	v := &sFile{store: s, cfg: c, drivers: map[string]storager.Driver{}}
	for name, cfg := range c.Storage.Drivers {
		if !cfg.Enabled {
			continue
		}
		d, e := storager.New(cfg, c.Server.BaseURL)
		if e != nil {
			v.Close()
			return nil, errors.New("invalid storage configuration: " + name + ": " + e.Error())
		}
		v.drivers[name] = d
	}
	if local, ok := v.drivers[c.Storage.Default].(*storager.Local); ok {
		v.Multipart = storager.NewMultipart(local, c.Storage.MaxBytes, time.Duration(c.Storage.MultipartSeconds)*time.Second, func(ctx context.Context, m storager.Manifest, r io.ReadSeeker) (int64, error) {
			var existing *view.Attachment
			if e := s.Files(ctx).Where("upload_id", m.ID).Where("owner_id", m.Owner).Scan(&existing); e != nil {
				return 0, e
			}
			if existing != nil {
				return existing.Id, nil
			}
			f, e := v.upload(ctx, m.Owner, m.Name, r, m.Size, m.Private, m.ID, m.Digest)
			if e != nil {
				return 0, e
			}
			return f.Id, nil
		})
	}
	return v, nil
}
func (s *sFile) Close() {
	for _, d := range s.drivers {
		if c, ok := d.(io.Closer); ok {
			c.Close()
		}
	}
}
func (s *sFile) Storage(name string) (storager.Driver, bool) { d, ok := s.drivers[name]; return d, ok }
func (s *sFile) Upload(ctx context.Context, owner int64, name string, r io.ReadSeeker, size int64, private bool) (*model.FileResult, error) {
	return s.upload(ctx, owner, name, r, size, private, "", "")
}
func (s *sFile) upload(ctx context.Context, owner int64, name string, r io.ReadSeeker, size int64, private bool, uploadID, expectedDigest string) (*model.FileResult, error) {
	if size <= 0 || size > s.cfg.Storage.MaxBytes {
		return nil, response.Fail(400, "FILE_SIZE_INVALID", "文件大小超出限制")
	}
	head := make([]byte, 512)
	n, e := r.Read(head)
	if e != nil && e != io.EOF {
		return nil, e
	}
	contentType, _, _ := mime.ParseMediaType(http.DetectContentType(head[:n]))
	allowed := false
	for _, m := range s.cfg.Storage.AllowedTypes {
		if m == contentType {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, response.Fail(400, "FILE_TYPE_INVALID", "不允许的文件类型")
	}
	extensions := map[string][]string{"image/jpeg": {".jpg", ".jpeg"}, "image/png": {".png"}, "image/gif": {".gif"}, "image/webp": {".webp"}, "application/pdf": {".pdf"}, "text/plain": {".txt"}, "application/zip": {".zip"}}
	ext := strings.ToLower(filepath.Ext(name))
	valid := false
	for _, v := range extensions[contentType] {
		if ext == v {
			valid = true
			break
		}
	}
	if !valid {
		return nil, response.Fail(400, "FILE_TYPE_INVALID", "文件扩展名与内容不匹配")
	}
	if _, e = r.Seek(0, io.SeekStart); e != nil {
		return nil, e
	}
	hash := sha256.New()
	actual, e := io.Copy(hash, io.LimitReader(r, size+1))
	if e != nil {
		return nil, e
	}
	if actual != size {
		return nil, response.Fail(400, "FILE_SIZE_INVALID", "文件大小不匹配")
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	if expectedDigest != "" && digest != expectedDigest {
		return nil, response.Fail(400, "FILE_DIGEST_MISMATCH", "完整文件校验失败")
	}
	if _, e = r.Seek(0, io.SeekStart); e != nil {
		return nil, e
	}
	b := make([]byte, 20)
	if _, e = rand.Read(b); e != nil {
		return nil, e
	}
	visibility := "private"
	if !private {
		visibility = "public"
	}
	key := visibility + "/" + time.Now().UTC().Format("2006/01/02") + "/" + hex.EncodeToString(b) + ext
	cfg := s.cfg.Storage.Drivers[s.cfg.Storage.Default]
	if cfg.Prefix != "" {
		if e = storager.SafeKey(cfg.Prefix); e != nil {
			return nil, e
		}
		key = cfg.Prefix + "/" + key
	}
	d := s.drivers[s.cfg.Storage.Default]
	if e = d.Put(ctx, key, r, size, contentType); e != nil {
		return nil, e
	}
	data := g.Map{"owner_id": owner, "storage_name": s.cfg.Storage.Default, "driver": cfg.Driver, "bucket": cfg.Bucket, "object_key": key, "name": filepath.Base(name), "mime_type": contentType, "size": size, "digest": digest, "private": private}
	if uploadID != "" {
		data["upload_id"] = uploadID
	}
	id, e := s.store.InsertID(ctx, dao.Attachments.Table(), data)
	if e != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if cleanupErr := d.Delete(cleanupCtx, key); cleanupErr != nil {
			g.Log().Errorf(cleanupCtx, "orphan upload storage=%s object=%s error=%v", s.cfg.Storage.Default, key, cleanupErr)
		}
		return nil, dao.Error(e)
	}
	f, e := s.Get(ctx, owner, id)
	if e != nil {
		return nil, e
	}
	return s.WithURL(ctx, f)
}
func (s *sFile) Get(ctx context.Context, owner, id int64) (*view.Attachment, error) {
	var f *view.Attachment
	e := s.store.Files(ctx).Where("id", id).Where("owner_id", owner).Scan(&f)
	if e != nil {
		return nil, e
	}
	if f == nil {
		return nil, response.Fail(404, "FILE_NOT_FOUND", "附件不存在")
	}
	return f, nil
}
func (s *sFile) GetForDownload(ctx context.Context, id int64) (*view.Attachment, error) {
	var f *view.Attachment
	e := s.store.Files(ctx).Where("id", id).Scan(&f)
	if e != nil {
		return nil, e
	}
	if f == nil {
		return nil, response.Fail(404, "FILE_NOT_FOUND", "附件不存在")
	}
	return f, nil
}
func (s *sFile) WithURL(ctx context.Context, f *view.Attachment) (*model.FileResult, error) {
	d, ok := s.drivers[f.StorageName]
	if !ok {
		return nil, response.Fail(503, "STORAGE_UNAVAILABLE", "附件原存储未配置")
	}
	cfg := s.cfg.Storage.Drivers[f.StorageName]
	if cfg.Driver != f.Driver || cfg.Bucket != f.Bucket {
		return nil, response.Fail(503, "STORAGE_CHANGED", "附件存储配置与记录不一致")
	}
	ttl := time.Duration(s.cfg.Storage.LinkSeconds) * time.Second
	var u string
	var e error
	if local, ok := d.(*storager.Local); ok {
		if f.Private {
			u, e = local.SignedURL(f.Id, ttl)
		} else {
			u = s.cfg.Server.BaseURL + "/api/v1/files/" + g.NewVar(f.Id).String() + "/content"
		}
	} else {
		u, e = d.URL(ctx, f.ObjectKey, f.Private, ttl)
	}
	if e != nil {
		return nil, e
	}
	r := &model.FileResult{Attachment: f, URL: u}
	if f.Private {
		r.ExpiresAt = time.Now().Add(ttl).Unix()
	}
	return r, nil
}
func (s *sFile) List(ctx context.Context, owner int64, p model.Page) (*model.List[*view.Attachment], error) {
	p = p.Normalized()
	m := s.store.Files(ctx).Where("owner_id", owner)
	n, e := m.Clone().Count()
	if e != nil {
		return nil, e
	}
	list := []*view.Attachment{}
	e = m.Clone().OrderDesc("id").Page(p.Page, p.PageSize).Scan(&list)
	return &model.List[*view.Attachment]{List: list, Total: n, Page: p.Page, PageSize: p.PageSize}, e
}
func (s *sFile) Delete(ctx context.Context, owner, id int64) error {
	return s.store.DB.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var f *view.Attachment
		if e := tx.Model("attachments").Ctx(ctx).Where("id", id).Where("owner_id", owner).LockUpdate().Scan(&f); e != nil {
			return e
		}
		if f == nil {
			return response.Fail(404, "FILE_NOT_FOUND", "附件不存在")
		}
		n, e := tx.Model("users").Ctx(ctx).Where("avatar_id", id).Count()
		if e != nil {
			return e
		}
		if n > 0 {
			return response.Fail(409, "FILE_IN_USE", "请先解除头像引用")
		}
		d, ok := s.drivers[f.StorageName]
		if !ok {
			return response.Fail(503, "STORAGE_UNAVAILABLE", "附件原存储未配置")
		}
		cfg := s.cfg.Storage.Drivers[f.StorageName]
		if cfg.Driver != f.Driver || cfg.Bucket != f.Bucket {
			return response.Fail(503, "STORAGE_CHANGED", "附件存储配置与记录不一致")
		}
		if e = d.Delete(ctx, f.ObjectKey); e != nil {
			return e
		}
		_, e = tx.Model("attachments").Ctx(ctx).Where("id", id).Delete()
		return dao.Error(e)
	})
}
func (s *sFile) Cleanup(ctx context.Context) error {
	for _, d := range s.drivers {
		if local, ok := d.(*storager.Local); ok {
			m := storager.NewMultipart(local, s.cfg.Storage.MaxBytes, time.Duration(s.cfg.Storage.MultipartSeconds)*time.Second, nil)
			if e := m.Cleanup(ctx); e != nil {
				return e
			}
		}
	}
	return nil
}

func (s *sFile) MultipartInit(owner int64, name string, size, chunk int64, digest string, private bool) (*storager.Manifest, error) {
	if s.Multipart == nil {
		return nil, response.Fail(501, "STORAGE_CAPABILITY_UNSUPPORTED", "当前默认存储不支持分片")
	}
	return s.Multipart.Init(owner, name, size, chunk, digest, private)
}
func (s *sFile) MultipartStatus(ctx context.Context, owner int64, id string) (*storager.Manifest, error) {
	if s.Multipart == nil {
		return nil, response.Fail(501, "STORAGE_CAPABILITY_UNSUPPORTED", "当前默认存储不支持分片")
	}
	return s.Multipart.Get(ctx, owner, id)
}
func (s *sFile) MultipartPart(ctx context.Context, owner int64, id string, index int, r io.Reader, size int64) error {
	if s.Multipart == nil {
		return response.Fail(501, "STORAGE_CAPABILITY_UNSUPPORTED", "当前默认存储不支持分片")
	}
	return s.Multipart.Part(ctx, owner, id, index, r, size)
}
func (s *sFile) MultipartComplete(ctx context.Context, owner int64, id string) (*model.FileResult, error) {
	if s.Multipart == nil {
		return nil, response.Fail(501, "STORAGE_CAPABILITY_UNSUPPORTED", "当前默认存储不支持分片")
	}
	fid, e := s.Multipart.Complete(ctx, owner, id)
	if e != nil {
		return nil, e
	}
	f, e := s.Get(ctx, owner, fid)
	if e != nil {
		return nil, e
	}
	return s.WithURL(ctx, f)
}
