package auth

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/auth/v1"
	core "github.com/reedtang666/go-easy-starter/internal/library/auth"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Login(ctx context.Context, req *v1.LoginReq) (*v1.LoginRes, error) {
	p, e := service.Auth().Login(ctx, req.Username, req.Password, core.Device{ID: req.DeviceId, Platform: req.Platform})
	return &v1.LoginRes{TokenPair: p}, e
}
