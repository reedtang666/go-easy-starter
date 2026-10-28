package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/reedtang666/go-easy-starter/internal/model"
	view "github.com/reedtang666/go-easy-starter/internal/model/view"
)

type ListReq struct {
	g.Meta `path:"/example/items" method:"get" tags:"示例" summary:"本人示例列表" security:"BearerToken"`
	model.ListQuery
}
type ListRes struct{ *model.List[*view.ExampleItem] }
type CreateReq struct {
	g.Meta  `path:"/example/items" method:"post" tags:"示例" summary:"创建示例" security:"BearerToken"`
	Title   string `json:"title" v:"required|length:1,200"`
	Content string `json:"content" v:"length:0,10000"`
}
type CreateRes struct {
	Item *view.ExampleItem `json:"item"`
}
type GetReq struct {
	g.Meta `path:"/example/items/{id}" method:"get" tags:"示例" summary:"查询本人示例" security:"BearerToken"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}
type GetRes = CreateRes
type UpdateReq struct {
	g.Meta  `path:"/example/items/{id}" method:"put" tags:"示例" summary:"更新本人示例" security:"BearerToken"`
	Id      int64  `json:"id" in:"path" v:"required|min:1"`
	Title   string `json:"title" v:"required|length:1,200"`
	Content string `json:"content" v:"length:0,10000"`
}
type UpdateRes = CreateRes
type DeleteReq struct {
	g.Meta `path:"/example/items/{id}" method:"delete" tags:"示例" summary:"删除本人示例" security:"BearerToken"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}
type DeleteRes struct{}
type TaskReq struct {
	Priority     string `json:"priority" d:"default" v:"in:default,high,low"`
	g.Meta       `path:"/example/tasks" method:"post" tags:"队列" summary:"投递日志或幂等示例任务" security:"BearerToken"`
	ItemId       int64  `json:"itemId" v:"min:0"`
	Key          string `json:"key" v:"length:0,100"`
	Message      string `json:"message" v:"length:0,500"`
	DelaySeconds int    `json:"delaySeconds" v:"between:0,604800"`
}
type TaskRes struct {
	TaskId string `json:"taskId"`
	Queue  string `json:"queue"`
}
