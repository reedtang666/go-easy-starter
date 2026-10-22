package auth

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/auth/v1"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Refresh(ctx context.Context, req *v1.RefreshReq) (*v1.RefreshRes, error) {
	p, e := service.Auth().Refresh(ctx, req.RefreshToken, req.DeviceId)
	return &v1.RefreshRes{TokenPair: p}, e
}
