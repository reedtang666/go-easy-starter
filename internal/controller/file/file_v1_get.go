package file

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/file/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Get(ctx context.Context, req *v1.GetReq) (*v1.GetRes, error) {
	f, e := service.File().Get(ctx, contexts.Get(ctx).UserID, req.Id)
	return &v1.GetRes{File: f}, e
}
