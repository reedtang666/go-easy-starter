package websocket

import (
	"context"
	"github.com/reedtang666/go-easy-starter/api/websocket/v1"
	"github.com/reedtang666/go-easy-starter/internal/library/contexts"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"github.com/reedtang666/go-easy-starter/internal/service"
)

func (c *ControllerV1) Ticket(ctx context.Context, req *v1.TicketReq) (*v1.TicketRes, error) {
	r := service.Current()
	if !r.Config.WebSocket.Enabled {
		return nil, response.Fail(404, "NOT_FOUND", "接口不存在")
	}
	ticket, e := r.Tickets.Issue(ctx, contexts.Get(ctx))
	return &v1.TicketRes{Ticket: ticket, ExpiresIn: r.Config.WebSocket.TicketSeconds}, e
}
