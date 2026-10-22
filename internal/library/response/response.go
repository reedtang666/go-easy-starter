package response

import (
	"errors"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/net/gtrace"
	"net/http"
)

type Error struct {
	Status  int
	Reason  string
	Message string
	Cause   error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }
func Fail(status int, reason, message string) error {
	return &Error{Status: status, Reason: reason, Message: message}
}

type Envelope struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Data    any    `json:"data"`
	TraceID string `json:"traceId"`
	Reason  string `json:"reason,omitempty"`
}

func Write(r *ghttp.Request, status int, reason, message string, data any) {
	code := 0
	if status >= 400 {
		code = status
	}
	r.Response.WriteStatus(status, Envelope{Code: code, Msg: message, Data: data, Reason: reason, TraceID: gtrace.GetTraceID(r.Context())})
}
func Handler(r *ghttp.Request) {
	r.Middleware.Next()
	if r.GetCtxVar("starter.raw").Bool() {
		return
	}
	if r.Response.BufferLength() > 0 || r.IsExited() {
		return
	}
	err := r.GetError()
	if err == nil {
		Write(r, 200, "", "OK", r.GetHandlerResponse())
		return
	}
	var e *Error
	if errors.As(err, &e) {
		if e.Cause != nil {
			g.Log().Errorf(r.Context(), "request failed reason=%s cause_type=%T trace=%s", e.Reason, e.Cause, gtrace.GetTraceID(r.Context()))
		}
		Write(r, e.Status, e.Reason, e.Message, nil)
		return
	}
	if gerror.Code(err) == gcode.CodeValidationFailed {
		Write(r, 400, "VALIDATION_FAILED", "请求参数无效", nil)
		return
	}
	g.Log().Errorf(r.Context(), "request failed method=%s path=%s trace=%s error_type=%T", r.Method, r.URL.Path, gtrace.GetTraceID(r.Context()), err)
	Write(r, http.StatusInternalServerError, "INTERNAL_ERROR", "服务暂时不可用", nil)
}
