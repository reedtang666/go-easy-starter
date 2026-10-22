package health

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/health/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Ready(ctx context.Context, req *v1.ReadyReq) (*v1.ReadyRes, error) {
	if e := service.Current().Ready(ctx); e != nil {
		return nil, response.Fail(503, "NOT_READY", "依赖服务未就绪")
	}
	return &v1.ReadyRes{Status: "ready"}, nil
}
