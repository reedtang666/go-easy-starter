// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"

	view "github.com/reedtang666/go-easy-starter/internal/model/view"
)

type (
	IUser interface {
		Register(ctx context.Context, username string, password string) (*view.User, error)
		Get(ctx context.Context, id int64) (*view.User, error)
		Authenticate(ctx context.Context, username string, password string) (*view.User, error)
		Update(ctx context.Context, id int64, nickname string, avatarID *int64) (*view.User, error)
		ChangePassword(ctx context.Context, id int64, old string, new string) error
	}
)

var (
	localUser IUser
)

func User() IUser {
	if localUser == nil {
		panic("implement not found for interface IUser, forgot register?")
	}
	return localUser
}

func RegisterUser(i IUser) {
	localUser = i
}
