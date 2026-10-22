package v1

import "github.com/gogf/gf/v2/frame/g"

type LiveReq struct {
	g.Meta `path:"/health/live" method:"get" tags:"健康检查" summary:"进程存活"`
}
type LiveRes struct {
	Status string `json:"status"`
}
type ReadyReq struct {
	g.Meta `path:"/health/ready" method:"get" tags:"健康检查" summary:"依赖就绪"`
}
type ReadyRes = LiveRes
