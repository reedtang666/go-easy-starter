// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package health

import (
	"context"

	"github.com/reedtang666/go-easy-starter/api/health/v1"
)

type IHealthV1 interface {
	Live(ctx context.Context, req *v1.LiveReq) (res *v1.LiveRes, err error)
	Ready(ctx context.Context, req *v1.ReadyReq) (res *v1.ReadyRes, err error)
}
