package example

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/example/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Get(ctx context.Context, req *v1.GetReq) (*v1.GetRes, error) {
	item, e := service.Example().Get(ctx, contexts.Get(ctx).UserID, req.Id)
	return &v1.GetRes{Item: item}, e
}
