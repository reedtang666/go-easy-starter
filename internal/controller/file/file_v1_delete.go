package file

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/file/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Delete(ctx context.Context, req *v1.DeleteReq) (*v1.DeleteRes, error) {
	return &v1.DeleteRes{}, service.File().Delete(ctx, contexts.Get(ctx).UserID, req.Id)
}
