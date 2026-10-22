package example

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/example/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Create(ctx context.Context, req *v1.CreateReq) (*v1.CreateRes, error) {
	item, e := service.Example().Create(ctx, contexts.Get(ctx).UserID, req.Title, req.Content)
	return &v1.CreateRes{Item: item}, e
}
