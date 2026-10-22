package file

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/file/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Upload(ctx context.Context, req *v1.UploadReq) (*v1.UploadRes, error) {
	f, e := req.File.Open()
	if e != nil {
		return nil, e
	}
	defer f.Close()
	result, e := service.File().Upload(ctx, contexts.Get(ctx).UserID, req.File.Filename, f, req.File.Size, req.Private)
	return &v1.UploadRes{File: result}, e
}
