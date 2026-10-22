// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// Users is the golang structure for table users.
type Users struct {
	Id                int64       `json:"id"                orm:"id"                 description:""`
	Username          string      `json:"username"          orm:"username"           description:""`
	PasswordHash      string      `json:"passwordHash"      orm:"password_hash"      description:""`
	Nickname          string      `json:"nickname"          orm:"nickname"           description:""`
	AvatarId          int64       `json:"avatarId"          orm:"avatar_id"          description:""`
	Status            int         `json:"status"            orm:"status"             description:""`
	CredentialVersion int64       `json:"credentialVersion" orm:"credential_version" description:""`
	CreatedAt         *gtime.Time `json:"createdAt"         orm:"created_at"         description:""`
	UpdatedAt         *gtime.Time `json:"updatedAt"         orm:"updated_at"         description:""`
}
