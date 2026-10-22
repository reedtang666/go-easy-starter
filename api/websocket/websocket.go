// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package websocket

import (
	"context"

	"github.com/reedtang666/go-easy-starter/api/websocket/v1"
)

type IWebsocketV1 interface {
	Ticket(ctx context.Context, req *v1.TicketReq) (res *v1.TicketRes, err error)
}
