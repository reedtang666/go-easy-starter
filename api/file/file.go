// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package file

import (
	"context"

	"github.com/reedtang666/go-easy-starter/api/file/v1"
)

type IFileV1 interface {
	Upload(ctx context.Context, req *v1.UploadReq) (res *v1.UploadRes, err error)
	List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error)
	Get(ctx context.Context, req *v1.GetReq) (res *v1.GetRes, err error)
	URL(ctx context.Context, req *v1.URLReq) (res *v1.URLRes, err error)
	Delete(ctx context.Context, req *v1.DeleteReq) (res *v1.DeleteRes, err error)
	MultipartInit(ctx context.Context, req *v1.MultipartInitReq) (res *v1.MultipartInitRes, err error)
	MultipartStatus(ctx context.Context, req *v1.MultipartStatusReq) (res *v1.MultipartStatusRes, err error)
	MultipartPart(ctx context.Context, req *v1.MultipartPartReq) (res *v1.MultipartPartRes, err error)
	MultipartComplete(ctx context.Context, req *v1.MultipartCompleteReq) (res *v1.MultipartCompleteRes, err error)
}
