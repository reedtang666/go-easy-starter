package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	view "github.com/reedtang666/go-easy-starter/internal/model/view"
)

type MeReq struct {
	g.Meta `path:"/user/me" method:"get" tags:"用户" summary:"本人资料" security:"BearerToken"`
}
type MeRes struct {
	User *view.User `json:"user"`
}
type UpdateReq struct {
	g.Meta   `path:"/user/me" method:"patch" tags:"用户" summary:"修改本人资料" security:"BearerToken"`
	Nickname *string `json:"nickname" v:"length:0,80"`
	AvatarId *int64  `json:"avatarId" v:"min:0"`
}
type UpdateRes = MeRes
type PasswordReq struct {
	g.Meta      `path:"/user/password" method:"put" tags:"用户" summary:"修改密码并使原会话失效" security:"BearerToken"`
	OldPassword string `json:"oldPassword" v:"required|length:8,72"`
	NewPassword string `json:"newPassword" v:"required|length:8,72"`
}
type PasswordRes struct{}
