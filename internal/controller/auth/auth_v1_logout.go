package auth

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/auth/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Logout(ctx context.Context, req *v1.LogoutReq) (*v1.LogoutRes, error) {
	return &v1.LogoutRes{}, service.Auth().Logout(ctx, contexts.Get(ctx))
}
