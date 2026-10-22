package example

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/example/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Update(ctx context.Context, req *v1.UpdateReq) (*v1.UpdateRes, error) {
	item, e := service.Example().Update(ctx, contexts.Get(ctx).UserID, req.Id, req.Title, req.Content)
	return &v1.UpdateRes{Item: item}, e
}
