package example

import (
	"context"
	"errors"
	"github.com/reedtang666/go-easy-starter/api/example/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"github.com/reedtang666/go-easy-starter/internal/queue"
	"github.com/reedtang666/go-easy-starter/internal/service"
	"time"
)

func (c *ControllerV1) Task(ctx context.Context, req *v1.TaskReq) (*v1.TaskRes, error) {
	owner := contexts.Get(ctx).UserID
	if req.ItemId > 0 {
		if _, e := service.Example().Get(ctx, owner, req.ItemId); e != nil {
			return nil, e
		}
	}
	info, e := service.Current().Tasks.Enqueue(ctx, queue.Payload{Priority: req.Priority, OwnerID: owner, ItemID: req.ItemId, Key: req.Key, Message: req.Message}, time.Duration(req.DelaySeconds)*time.Second)
	if e != nil {
		var invalid *response.Error
		if errors.As(e, &invalid) {
			return nil, e
		}
		return nil, &response.Error{Status: 503, Reason: "QUEUE_UNAVAILABLE", Message: "队列暂时不可用", Cause: e}
	}
	return &v1.TaskRes{TaskId: info.ID, Queue: info.Queue}, nil
}
