package file

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/file/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) MultipartComplete(ctx context.Context, req *v1.MultipartCompleteReq) (*v1.MultipartCompleteRes, error) {
	f, e := service.File().MultipartComplete(ctx, contexts.Get(ctx).UserID, req.UploadId)
	return &v1.MultipartCompleteRes{File: f}, e
}
