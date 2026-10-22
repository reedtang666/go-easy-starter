// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package user

import (
	"context"

	"github.com/reedtang666/go-easy-starter/api/user/v1"
)

type IUserV1 interface {
	Me(ctx context.Context, req *v1.MeReq) (res *v1.MeRes, err error)
	Update(ctx context.Context, req *v1.UpdateReq) (res *v1.UpdateRes, err error)
	Password(ctx context.Context, req *v1.PasswordReq) (res *v1.PasswordRes, err error)
}
