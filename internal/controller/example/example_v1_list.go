package example

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/example/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) List(ctx context.Context, req *v1.ListReq) (*v1.ListRes, error) {
	list, e := service.Example().List(ctx, contexts.Get(ctx).UserID, req.Page)
	return &v1.ListRes{List: list}, e
}
