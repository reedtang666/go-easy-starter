package user

import (
	"context"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/reedtang666/go-easy-starter/api/user/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Password(ctx context.Context, req *v1.PasswordReq) (*v1.PasswordRes, error) {
	id := contexts.Get(ctx)
	if e := service.User().ChangePassword(ctx, id.UserID, req.OldPassword, req.NewPassword); e != nil {
		return nil, e
	}
	if e := service.Auth().Logout(ctx, id); e != nil {
		g.Log().Warning(ctx, "password changed; Redis session cleanup deferred, old credential version is revoked")
	}
	return &v1.PasswordRes{}, nil
}
