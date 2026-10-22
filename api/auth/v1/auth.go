package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	core "github.com/reedtang666/go-easy-starter/internal/library/auth"
	view "github.com/reedtang666/go-easy-starter/internal/model/view"
)

type RegisterReq struct {
	g.Meta   `path:"/auth/register" method:"post" tags:"认证" summary:"注册"`
	Username string `json:"username" v:"required|length:3,32"`
	Password string `json:"password" v:"required|length:8,72"`
}
type RegisterRes struct {
	User *view.User `json:"user"`
}
type LoginReq struct {
	g.Meta   `path:"/auth/login" method:"post" tags:"认证" summary:"账号密码登录"`
	Username string `json:"username" v:"required|length:3,32"`
	Password string `json:"password" v:"required|length:8,72"`
	DeviceId string `json:"deviceId" v:"required|length:1,128"`
	Platform string `json:"platform" v:"length:0,32"`
}
type LoginRes struct{ *core.TokenPair }
type RefreshReq struct {
	g.Meta       `path:"/auth/refresh" method:"post" tags:"认证" summary:"刷新令牌"`
	RefreshToken string `json:"refreshToken" v:"required|length:1,512"`
	DeviceId     string `json:"deviceId" v:"required|length:1,128"`
}
type RefreshRes struct{ *core.TokenPair }
type LogoutReq struct {
	g.Meta `path:"/auth/logout" method:"post" tags:"认证" summary:"退出当前会话" security:"BearerToken"`
}
type LogoutRes struct{}
