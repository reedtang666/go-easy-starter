package auth

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/auth/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Register(ctx context.Context, req *v1.RegisterReq) (*v1.RegisterRes, error) {
	if !service.Current().Config.Auth.Register {
		return nil, response.Fail(403, "REGISTRATION_DISABLED", "注册已关闭")
	}
	u, e := service.User().Register(ctx, req.Username, req.Password)
	return &v1.RegisterRes{User: u}, e
}
