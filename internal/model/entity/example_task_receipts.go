// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// ExampleTaskReceipts is the golang structure for table example_task_receipts.
type ExampleTaskReceipts struct {
	TaskKey   string      `json:"taskKey"   orm:"task_key"   description:""`
	ItemId    int64       `json:"itemId"    orm:"item_id"    description:""`
	CreatedAt *gtime.Time `json:"createdAt" orm:"created_at" description:""`
}
