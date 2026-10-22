package file

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/file/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) MultipartInit(ctx context.Context, req *v1.MultipartInitReq) (*v1.MultipartInitRes, error) {
	v, e := service.File().MultipartInit(contexts.Get(ctx).UserID, req.Name, req.Size, req.ChunkSize, req.Digest, req.Private)
	return &v1.MultipartInitRes{Upload: v}, e
}
