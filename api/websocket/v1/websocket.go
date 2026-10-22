package v1

import "github.com/gogf/gf/v2/frame/g"

type TicketReq struct {
	g.Meta `path:"/ws/ticket" method:"post" tags:"WebSocket" summary:"签发一次性连接票据" security:"BearerToken"`
}
type TicketRes struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expiresIn"`
}
