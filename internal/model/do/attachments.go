// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// Attachments is the golang structure of table attachments for DAO operations like Where/Data.
type Attachments struct {
	g.Meta      `orm:"table:attachments, do:true"`
	Id          interface{} //
	OwnerId     interface{} //
	StorageName interface{} //
	Driver      interface{} //
	Bucket      interface{} //
	ObjectKey   interface{} //
	Name        interface{} //
	MimeType    interface{} //
	Size        interface{} //
	Digest      interface{} //
	UploadId    interface{} //
	Private     interface{} //
	CreatedAt   *gtime.Time //
}
