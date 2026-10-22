// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// ExampleItems is the golang structure for table example_items.
type ExampleItems struct {
	Id        int64       `json:"id"        orm:"id"         description:""`
	OwnerId   int64       `json:"ownerId"   orm:"owner_id"   description:""`
	Title     string      `json:"title"     orm:"title"      description:""`
	Content   string      `json:"content"   orm:"content"    description:""`
	TaskRuns  int         `json:"taskRuns"  orm:"task_runs"  description:""`
	CreatedAt *gtime.Time `json:"createdAt" orm:"created_at" description:""`
	UpdatedAt *gtime.Time `json:"updatedAt" orm:"updated_at" description:""`
}
