// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ExampleTaskReceipts is the golang structure of table example_task_receipts for DAO operations like Where/Data.
type ExampleTaskReceipts struct {
	g.Meta    `orm:"table:example_task_receipts, do:true"`
	TaskKey   interface{} //
	ItemId    interface{} //
	CreatedAt *gtime.Time //
}
