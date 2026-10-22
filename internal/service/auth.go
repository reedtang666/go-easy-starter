// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"

	core "github.com/reedtang666/go-easy-starter/internal/library/auth"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
)

type (
	IAuth interface {
		Login(ctx context.Context, username string, password string, device core.Device) (*core.TokenPair, error)
		Refresh(ctx context.Context, value string, device string) (*core.TokenPair, error)
		Validate(ctx context.Context, value string) (contexts.Identity, error)
		ValidateSession(ctx context.Context, id contexts.Identity) error
		Logout(ctx context.Context, id contexts.Identity) error
	}
)

var (
	localAuth IAuth
)

func Auth() IAuth {
	if localAuth == nil {
		panic("implement not found for interface IAuth, forgot register?")
	}
	return localAuth
}

func RegisterAuth(i IAuth) {
	localAuth = i
}
