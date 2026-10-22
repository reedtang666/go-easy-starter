package file

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/file/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) URL(ctx context.Context, req *v1.URLReq) (*v1.URLRes, error) {
	f, e := service.File().Get(ctx, contexts.Get(ctx).UserID, req.Id)
	if e != nil {
		return nil, e
	}
	result, e := service.File().WithURL(ctx, f)
	return &v1.URLRes{File: result}, e
}
