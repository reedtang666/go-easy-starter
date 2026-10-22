package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/reedtang666/go-easy-starter/internal/library/storager"
	"github.com/reedtang666/go-easy-starter/internal/model"
	view "github.com/reedtang666/go-easy-starter/internal/model/view"
)

type UploadReq struct {
	g.Meta  `path:"/files" method:"post" tags:"文件" summary:"上传文件" security:"BearerToken"`
	File    *ghttp.UploadFile `json:"file" type:"file" mime:"file" v:"required"`
	Private bool              `json:"private" d:"true"`
}
type UploadRes struct {
	File *model.FileResult `json:"file"`
}
type ListReq struct {
	g.Meta `path:"/files" method:"get" tags:"文件" summary:"本人附件列表" security:"BearerToken"`
	model.Page
}
type ListRes struct{ *model.List[*view.Attachment] }
type GetReq struct {
	g.Meta `path:"/files/{id}" method:"get" tags:"文件" summary:"本人附件元数据" security:"BearerToken"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}
type GetRes struct {
	File *view.Attachment `json:"file"`
}
type URLReq struct {
	g.Meta `path:"/files/{id}/url" method:"get" tags:"文件" summary:"获取访问地址" security:"BearerToken"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}
type URLRes = UploadRes
type DeleteReq struct {
	g.Meta `path:"/files/{id}" method:"delete" tags:"文件" summary:"删除本人附件" security:"BearerToken"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}
type DeleteRes struct{}
type MultipartInitReq struct {
	g.Meta    `path:"/files/multipart" method:"post" tags:"文件" summary:"初始化本地分片" security:"BearerToken"`
	Name      string `json:"name" v:"required|length:1,255"`
	Size      int64  `json:"size" v:"required|min:1"`
	ChunkSize int64  `json:"chunkSize" d:"1048576" v:"between:65536,4194304"`
	Digest    string `json:"digest" v:"required|length:64,64"`
	Private   bool   `json:"private" d:"true"`
}
type MultipartInitRes struct {
	Upload *storager.Manifest `json:"upload"`
}
type MultipartStatusReq struct {
	g.Meta   `path:"/files/multipart/{uploadId}" method:"get" tags:"文件" summary:"分片进度" security:"BearerToken"`
	UploadId string `json:"uploadId" in:"path" v:"required|length:40,40"`
}
type MultipartStatusRes = MultipartInitRes
type MultipartPartReq struct {
	g.Meta   `path:"/files/multipart/{uploadId}/parts/{index}" method:"put" tags:"文件" summary:"上传分片" security:"BearerToken"`
	UploadId string            `json:"uploadId" in:"path" v:"required|length:40,40"`
	Index    int               `json:"index" in:"path" v:"min:0"`
	File     *ghttp.UploadFile `json:"file" mime:"file" type:"file" v:"required"`
}
type MultipartPartRes struct{}
type MultipartCompleteReq struct {
	g.Meta   `path:"/files/multipart/{uploadId}/complete" method:"post" tags:"文件" summary:"完成本地分片" security:"BearerToken"`
	UploadId string `json:"uploadId" in:"path" v:"required|length:40,40"`
}
type MultipartCompleteRes = UploadRes
