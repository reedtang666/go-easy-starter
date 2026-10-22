package health

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/health/v1"
)

func (c *ControllerV1) Live(ctx context.Context, req *v1.LiveReq) (*v1.LiveRes, error) {
	return &v1.LiveRes{Status: "alive"}, nil
}
