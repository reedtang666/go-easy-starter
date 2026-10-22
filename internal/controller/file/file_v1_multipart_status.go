package file

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/file/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) MultipartStatus(ctx context.Context, req *v1.MultipartStatusReq) (*v1.MultipartStatusRes, error) {
	v, e := service.File().MultipartStatus(ctx, contexts.Get(ctx).UserID, req.UploadId)
	return &v1.MultipartStatusRes{Upload: v}, e
}
