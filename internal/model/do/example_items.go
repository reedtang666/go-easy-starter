// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ExampleItems is the golang structure of table example_items for DAO operations like Where/Data.
type ExampleItems struct {
	g.Meta    `orm:"table:example_items, do:true"`
	Id        interface{} //
	OwnerId   interface{} //
	Title     interface{} //
	Content   interface{} //
	TaskRuns  interface{} //
	CreatedAt *gtime.Time //
	UpdatedAt *gtime.Time //
}
