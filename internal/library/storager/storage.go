package storager

import (
	"context"
	"errors"
	"fmt"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"io"
	"net/url"
	"path"
	"strings"
	"time"
)

var ErrUnsupported = errors.New("STORAGE_CAPABILITY_UNSUPPORTED")

type Driver interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Delete(context.Context, string) error
	URL(context.Context, string, bool, time.Duration) (string, error)
}

func SafeKey(key string) error {
	if key == "" || strings.HasPrefix(key, "/") || strings.ContainsAny(key, "\\\x00") || path.Clean(key) != key || key == "." || key == ".." || strings.HasPrefix(key, "../") {
		return errors.New("invalid object key")
	}
	return nil
}
func New(c config.Storage, baseURL string) (Driver, error) {
	if c.Driver == "local" {
		return NewLocal(c, baseURL)
	}
	switch c.Driver {
	case "oss", "cos", "qiniu", "minio", "ucloud":
	default:
		return nil, fmt.Errorf("unsupported storage driver %q", c.Driver)
	}
	if c.Bucket == "" || c.AccessKey == "" || c.SecretKey == "" {
		return nil, errors.New("cloud storage requires bucket, accessKey and secretKey")
	}
	if c.Driver == "qiniu" && c.PublicURL == "" {
		return nil, errors.New("qiniu requires publicUrl for download URLs")
	}
	if c.Driver != "qiniu" && c.Endpoint == "" {
		return nil, errors.New("cloud storage requires endpoint")
	}
	if c.Driver == "cos" {
		u, e := url.Parse(c.Endpoint)
		if e != nil || u.Host == "" {
			return nil, errors.New("cos endpoint must be bucket URL")
		}
	}
	return &Cloud{cfg: c}, nil
}
func PublicURL(base, key string) (string, error) {
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("publicUrl must be an HTTP URL")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + key
	return u.String(), nil
}
