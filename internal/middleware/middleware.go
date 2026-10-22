package middleware

import (
	"crypto/sha256"
	"fmt"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/net/gtrace"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"github.com/reedtang666/go-easy-starter/internal/service"
	"net"
	"net/http"
	"strings"
	"time"
)

func Common(c config.Config) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		start := time.Now()
		defer func() {
			g.Log().Infof(r.Context(), "request method=%s path=%s status=%d duration=%s trace=%s", r.Method, r.URL.Path, r.Response.Status, time.Since(start), gtrace.GetTraceID(r.Context()))
		}()
		r.Response.Header().Set("X-Content-Type-Options", "nosniff")
		origin := r.Header.Get("Origin")
		if origin != "" {
			allowed := false
			for _, v := range c.Server.Origins {
				if origin == v {
					allowed = true
				}
			}
			if !allowed {
				response.Write(r, 403, "ORIGIN_DENIED", "来源未被允许", nil)
				return
			}
			r.Response.Header().Set("Access-Control-Allow-Origin", origin)
			r.Response.Header().Set("Vary", "Origin")
			r.Response.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			r.Response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			r.Response.WriteStatus(204)
			return
		}
		r.Request.Body = http.MaxBytesReader(r.Response.ResponseWriter, r.Request.Body, c.Server.MaxBodyBytes)
		r.Middleware.Next()
	}
}
func Auth(r *ghttp.Request) {
	h := strings.Fields(r.Header.Get("Authorization"))
	if len(h) != 2 || !strings.EqualFold(h[0], "Bearer") || len(h[1]) > 8192 {
		response.Write(r, 401, "ACCESS_TOKEN_INVALID", "请提供有效登录凭证", nil)
		return
	}
	id, e := service.Auth().Validate(r.Context(), h[1])
	if e != nil {
		r.SetError(e)
		return
	}
	r.SetCtx(contexts.WithIdentity(r.Context(), id))
	r.Middleware.Next()
}
func Limit(redis *gredis.Redis, prefix string, max int64) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		id := contexts.Get(r.Context()).UserID
		key := fmt.Sprintf("%srate:%x", prefix, sha256.Sum256([]byte(fmt.Sprintf("%s:%d", host, id))))
		v, e := redis.Eval(r.Context(), `local n=redis.call('INCR',KEYS[1]);if n==1 then redis.call('EXPIRE',KEYS[1],60) end;return n`, 1, []string{key}, nil)
		if e != nil {
			response.Write(r, 503, "RATE_LIMIT_UNAVAILABLE", "服务暂时不可用", nil)
			return
		}
		if v.Int64() > max {
			r.Response.Header().Set("Retry-After", "60")
			response.Write(r, 429, "RATE_LIMITED", "请求过于频繁", nil)
			return
		}
		r.Middleware.Next()
	}
}
