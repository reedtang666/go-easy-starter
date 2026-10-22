// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
	"io"

	"github.com/reedtang666/go-easy-starter/internal/library/storager"
	"github.com/reedtang666/go-easy-starter/internal/model"
	view "github.com/reedtang666/go-easy-starter/internal/model/view"
)

type (
	IFile interface {
		Close()
		Storage(name string) (storager.Driver, bool)
		Upload(ctx context.Context, owner int64, name string, r io.ReadSeeker, size int64, private bool) (*model.FileResult, error)
		Get(ctx context.Context, owner int64, id int64) (*view.Attachment, error)
		GetForDownload(ctx context.Context, id int64) (*view.Attachment, error)
		WithURL(ctx context.Context, f *view.Attachment) (*model.FileResult, error)
		List(ctx context.Context, owner int64, p model.Page) (*model.List[*view.Attachment], error)
		Delete(ctx context.Context, owner int64, id int64) error
		Cleanup(ctx context.Context) error
		MultipartInit(owner int64, name string, size int64, chunk int64, digest string, private bool) (*storager.Manifest, error)
		MultipartStatus(ctx context.Context, owner int64, id string) (*storager.Manifest, error)
		MultipartPart(ctx context.Context, owner int64, id string, index int, r io.Reader, size int64) error
		MultipartComplete(ctx context.Context, owner int64, id string) (*model.FileResult, error)
	}
)

var (
	localFile IFile
)

func File() IFile {
	if localFile == nil {
		panic("implement not found for interface IFile, forgot register?")
	}
	return localFile
}

func RegisterFile(i IFile) {
	localFile = i
}
