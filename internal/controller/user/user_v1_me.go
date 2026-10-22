package user

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/user/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Me(ctx context.Context, req *v1.MeReq) (*v1.MeRes, error) {
	u, e := service.User().Get(ctx, contexts.Get(ctx).UserID)
	return &v1.MeRes{User: u}, e
}
