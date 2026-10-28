// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"

	"github.com/reedtang666/go-easy-starter/internal/model"
	view "github.com/reedtang666/go-easy-starter/internal/model/view"
)

type (
	IExample interface {
		Get(ctx context.Context, owner int64, id int64) (*view.ExampleItem, error)
		List(ctx context.Context, owner int64, input model.ListQuery) (*model.List[*view.ExampleItem], error)
		Create(ctx context.Context, owner int64, title string, content string) (*view.ExampleItem, error)
		Update(ctx context.Context, owner int64, id int64, title string, content string) (*view.ExampleItem, error)
		Delete(ctx context.Context, owner int64, id int64) error
		ApplyTask(ctx context.Context, owner int64, id int64, key string) error
	}
)

var (
	localExample IExample
)

func Example() IExample {
	if localExample == nil {
		panic("implement not found for interface IExample, forgot register?")
	}
	return localExample
}

func RegisterExample(i IExample) {
	localExample = i
}
