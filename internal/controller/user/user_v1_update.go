package user

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/user/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Update(ctx context.Context, req *v1.UpdateReq) (*v1.UpdateRes, error) {
	id := contexts.Get(ctx).UserID
	u, e := service.User().Get(ctx, id)
	if e != nil {
		return nil, e
	}
	nickname := u.Nickname
	if req.Nickname != nil {
		nickname = *req.Nickname
	}
	u, e = service.User().Update(ctx, id, nickname, req.AvatarId)
	return &v1.UpdateRes{User: u}, e
}
