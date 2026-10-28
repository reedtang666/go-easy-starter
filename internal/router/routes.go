package router

import (
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/reedtang666/go-easy-starter/internal/config"
	authctrl "github.com/reedtang666/go-easy-starter/internal/controller/auth"
	examplectrl "github.com/reedtang666/go-easy-starter/internal/controller/example"
	filectrl "github.com/reedtang666/go-easy-starter/internal/controller/file"
	healthctrl "github.com/reedtang666/go-easy-starter/internal/controller/health"
	userctrl "github.com/reedtang666/go-easy-starter/internal/controller/user"
	wsctrl "github.com/reedtang666/go-easy-starter/internal/controller/websocket"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"github.com/reedtang666/go-easy-starter/internal/library/storager"
	"github.com/reedtang666/go-easy-starter/internal/middleware"
	"github.com/reedtang666/go-easy-starter/internal/service"
	"mime"
	"strconv"
)

func Register(s *ghttp.Server, c config.Config, redis *gredis.Redis) {
	for _, status := range []int{404, 405, 413} {
		code := status
		s.BindStatusHandler(code, func(r *ghttp.Request) {
			if r.Response.BufferLength() > 0 {
				return
			}
			response.Write(r, code, "HTTP_ERROR", "请求无法处理", nil)
		})
	}
	s.Group("/", func(root *ghttp.RouterGroup) {
		root.Middleware(middleware.Common(c), response.Handler)
		root.Bind(healthctrl.NewV1())
		root.GET("/api/v1/files/{id}/content", download)
		if c.WebSocket.Enabled {
			root.GET("/ws", func(r *ghttp.Request) {
				r.SetCtxVar("starter.raw", true)
				service.Current().Hub.Serve(r.Response.ResponseWriter, r.Request)
			})
		}
		root.Group("/api/v1", func(api *ghttp.RouterGroup) {
			a := authctrl.NewV1()
			api.Group("/", func(public *ghttp.RouterGroup) {
				public.Middleware(middleware.Limit(redis, c.Redis.Auth.Prefix+"auth:", c.Limits.AuthPerMinute))
				public.Bind(a.Register, a.Login, a.Refresh)
			})
			api.Group("/", func(private *ghttp.RouterGroup) {
				private.Middleware(middleware.Auth)
				private.Bind(userctrl.NewV1(), examplectrl.NewV1(), a.Logout)
				registerModules(private)
				if c.WebSocket.Enabled {
					private.Bind(wsctrl.NewV1())
				}
				private.Group("/", func(files *ghttp.RouterGroup) {
					files.Middleware(middleware.Limit(redis, c.Redis.Auth.Prefix+"file:", c.Limits.UploadPerMinute))
					files.Bind(filectrl.NewV1())
				})
			})
		})
	})
}
func download(r *ghttp.Request) {
	id, e := strconv.ParseInt(r.GetRouter("id").String(), 10, 64)
	if e != nil || id <= 0 {
		r.SetError(response.Fail(404, "FILE_NOT_FOUND", "附件不存在"))
		return
	}
	f, e := service.File().GetForDownload(r.Context(), id)
	if e != nil {
		r.SetError(e)
		return
	}
	d, ok := service.File().Storage(f.StorageName)
	if !ok {
		r.SetError(response.Fail(503, "STORAGE_UNAVAILABLE", "附件原存储未配置"))
		return
	}
	local, ok := d.(*storager.Local)
	if !ok {
		r.SetError(response.Fail(404, "FILE_NOT_FOUND", "请通过签名地址访问对象存储"))
		return
	}
	if f.Private {
		expiry, e := strconv.ParseInt(r.GetQuery("expires").String(), 10, 64)
		if e != nil || !local.Verify(id, expiry, r.GetQuery("signature").String()) {
			r.SetError(response.Fail(403, "FILE_SIGNATURE_INVALID", "下载链接已失效"))
			return
		}
	}
	reader, e := local.Open(f.ObjectKey)
	if e != nil {
		r.SetError(response.Fail(404, "FILE_NOT_FOUND", "文件不存在"))
		return
	}
	defer reader.Close()
	r.Response.Header().Set("Content-Type", f.MimeType)
	r.Response.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Name}))
	if f.Private {
		r.Response.Header().Set("Cache-Control", "private, no-store")
	}
	r.SetCtxVar("starter.raw", true)
	r.Response.ServeContent(f.Name, f.CreatedAt, reader)
}
