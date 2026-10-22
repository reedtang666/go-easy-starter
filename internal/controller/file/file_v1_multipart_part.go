package file

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/file/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) MultipartPart(ctx context.Context, req *v1.MultipartPartReq) (*v1.MultipartPartRes, error) {
	f, e := req.File.Open()
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return &v1.MultipartPartRes{}, service.File().MultipartPart(ctx, contexts.Get(ctx).UserID, req.UploadId, req.Index, f, req.File.Size)
}
