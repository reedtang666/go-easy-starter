// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// Attachments is the golang structure for table attachments.
type Attachments struct {
	Id          int64       `json:"id"          orm:"id"           description:""`
	OwnerId     int64       `json:"ownerId"     orm:"owner_id"     description:""`
	StorageName string      `json:"storageName" orm:"storage_name" description:""`
	Driver      string      `json:"driver"      orm:"driver"       description:""`
	Bucket      string      `json:"bucket"      orm:"bucket"       description:""`
	ObjectKey   string      `json:"objectKey"   orm:"object_key"   description:""`
	Name        string      `json:"name"        orm:"name"         description:""`
	MimeType    string      `json:"mimeType"    orm:"mime_type"    description:""`
	Size        int64       `json:"size"        orm:"size"         description:""`
	Digest      string      `json:"digest"      orm:"digest"       description:""`
	UploadId    string      `json:"uploadId"    orm:"upload_id"    description:""`
	Private     bool        `json:"private"     orm:"private"      description:""`
	CreatedAt   *gtime.Time `json:"createdAt"   orm:"created_at"   description:""`
}
