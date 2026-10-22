package storager

import (
	"context"
	"errors"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	qiniu "github.com/qiniu/go-sdk/v7/storage"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"github.com/tencentyun/cos-go-sdk-v5"
	ufile "github.com/ufilesdk-dev/ufile-gosdk"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Cloud struct{ cfg config.Storage }

func (c *Cloud) ossBucket() (*oss.Bucket, error) {
	v, e := oss.New(c.cfg.Endpoint, c.cfg.AccessKey, c.cfg.SecretKey, oss.Timeout(10, 60))
	if e != nil {
		return nil, e
	}
	return v.Bucket(c.cfg.Bucket)
}
func (c *Cloud) cosClient() (*cos.Client, error) {
	u, e := url.Parse(c.cfg.Endpoint)
	if e != nil {
		return nil, e
	}
	return cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{Timeout: 60 * time.Second, Transport: &cos.AuthorizationTransport{SecretID: c.cfg.AccessKey, SecretKey: c.cfg.SecretKey}}), nil
}
func (c *Cloud) minioClient() (*minio.Client, error) {
	return minio.New(c.cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(c.cfg.AccessKey, c.cfg.SecretKey, ""), Secure: c.cfg.Secure})
}
func (c *Cloud) ucloudClient(ctx context.Context) (*ufile.UFileRequest, error) {
	return ufile.NewFileRequest(&ufile.Config{PublicKey: c.cfg.AccessKey, PrivateKey: c.cfg.SecretKey, BucketName: c.cfg.Bucket, BucketHost: c.cfg.BucketHost, FileHost: c.cfg.FileHost, Endpoint: c.cfg.Endpoint}, &http.Client{Timeout: 60 * time.Second, Transport: &requestTransport{ctx: ctx}})
}

type requestTransport struct{ ctx context.Context }

func (t *requestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return http.DefaultTransport.RoundTrip(r.WithContext(t.ctx))
}
func (c *Cloud) Put(ctx context.Context, key string, r io.Reader, size int64, mime string) error {
	if e := SafeKey(key); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	switch c.cfg.Driver {
	case "oss":
		b, e := c.ossBucket()
		if e != nil {
			return e
		}
		return b.PutObject(key, r, oss.ContentType(mime), oss.WithContext(ctx))
	case "cos":
		cl, e := c.cosClient()
		if e != nil {
			return e
		}
		_, e = cl.Object.Put(ctx, key, r, &cos.ObjectPutOptions{ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: mime, ContentLength: size}})
		return e
	case "qiniu":
		cfg := qiniu.Config{UseHTTPS: true}
		region, e := qiniu.GetRegion(c.cfg.AccessKey, c.cfg.Bucket)
		if e != nil {
			return e
		}
		cfg.Region = region
		token := (&qiniu.PutPolicy{Scope: c.cfg.Bucket}).UploadToken(qbox.NewMac(c.cfg.AccessKey, c.cfg.SecretKey))
		return qiniu.NewFormUploader(&cfg).Put(ctx, &qiniu.PutRet{}, token, key, r, size, &qiniu.PutExtra{MimeType: mime})
	case "minio":
		cl, e := c.minioClient()
		if e != nil {
			return e
		}
		_, e = cl.PutObject(ctx, c.cfg.Bucket, key, r, size, minio.PutObjectOptions{ContentType: mime})
		return e
	case "ucloud":
		cl, e := c.ucloudClient(ctx)
		if e != nil {
			return e
		}
		return cl.IOPut(r, key, mime)
	}
	return ErrUnsupported
}
func (c *Cloud) Delete(ctx context.Context, key string) error {
	if e := SafeKey(key); e != nil {
		return e
	}
	switch c.cfg.Driver {
	case "oss":
		b, e := c.ossBucket()
		if e != nil {
			return e
		}
		e = b.DeleteObject(key, oss.WithContext(ctx))
		var se oss.ServiceError
		if errors.As(e, &se) && se.StatusCode == 404 {
			return nil
		}
		return e
	case "cos":
		cl, e := c.cosClient()
		if e != nil {
			return e
		}
		resp, e := cl.Object.Delete(ctx, key)
		if resp != nil && resp.StatusCode == 404 {
			return nil
		}
		return e
	case "qiniu":
		cl := qiniu.NewBucketManager(qbox.NewMac(c.cfg.AccessKey, c.cfg.SecretKey), &qiniu.Config{UseHTTPS: true})
		e := cl.Delete(c.cfg.Bucket, key)
		if ei, ok := e.(*qiniu.ErrorInfo); ok && ei.Code == 612 {
			return nil
		}
		return e
	case "minio":
		cl, e := c.minioClient()
		if e != nil {
			return e
		}
		e = cl.RemoveObject(ctx, c.cfg.Bucket, key, minio.RemoveObjectOptions{})
		if minio.ToErrorResponse(e).Code == "NoSuchKey" {
			return nil
		}
		return e
	case "ucloud":
		cl, e := c.ucloudClient(ctx)
		if e != nil {
			return e
		}
		e = cl.DeleteFile(key)
		if cl.LastResponseStatus == 404 {
			return nil
		}
		return e
	}
	return ErrUnsupported
}
func (c *Cloud) URL(ctx context.Context, key string, private bool, ttl time.Duration) (string, error) {
	if e := SafeKey(key); e != nil {
		return "", e
	}
	if !private {
		base := c.cfg.PublicURL
		if base == "" {
			switch c.cfg.Driver {
			case "cos":
				base = c.cfg.Endpoint
			case "minio":
				scheme := "http://"
				if c.cfg.Secure {
					scheme = "https://"
				}
				base = scheme + c.cfg.Endpoint + "/" + c.cfg.Bucket
			case "oss":
				u, e := url.Parse(c.cfg.Endpoint)
				if e != nil {
					return "", e
				}
				u.Host = c.cfg.Bucket + "." + u.Host
				base = u.String()
			case "ucloud":
				cl, e := c.ucloudClient(ctx)
				if e != nil {
					return "", e
				}
				return cl.GetPublicURL(key), nil
			}
		}
		return PublicURL(base, key)
	}
	if ttl <= 0 {
		return "", errors.New("invalid URL lifetime")
	}
	switch c.cfg.Driver {
	case "oss":
		b, e := c.ossBucket()
		if e != nil {
			return "", e
		}
		return b.SignURL(key, oss.HTTPGet, int64(ttl/time.Second))
	case "cos":
		cl, e := c.cosClient()
		if e != nil {
			return "", e
		}
		u, e := cl.Object.GetPresignedURL(ctx, http.MethodGet, key, c.cfg.AccessKey, c.cfg.SecretKey, ttl, nil)
		if e != nil {
			return "", e
		}
		return u.String(), nil
	case "qiniu":
		return qiniu.MakePrivateURL(qbox.NewMac(c.cfg.AccessKey, c.cfg.SecretKey), strings.TrimRight(c.cfg.PublicURL, "/"), key, time.Now().Add(ttl).Unix()), nil
	case "minio":
		cl, e := c.minioClient()
		if e != nil {
			return "", e
		}
		u, e := cl.PresignedGetObject(ctx, c.cfg.Bucket, key, ttl, nil)
		if e != nil {
			return "", e
		}
		return u.String(), nil
	case "ucloud":
		cl, e := c.ucloudClient(ctx)
		if e != nil {
			return "", e
		}
		return cl.GetPrivateURL(key, ttl), nil
	}
	return "", ErrUnsupported
}
