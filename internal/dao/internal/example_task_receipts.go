// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ExampleTaskReceiptsDao is the data access object for table example_task_receipts.
type ExampleTaskReceiptsDao struct {
	table   string                     // table is the underlying table name of the DAO.
	group   string                     // group is the database configuration group name of current DAO.
	columns ExampleTaskReceiptsColumns // columns contains all the column names of Table for convenient usage.
}

// ExampleTaskReceiptsColumns defines and stores column names for table example_task_receipts.
type ExampleTaskReceiptsColumns struct {
	TaskKey   string //
	ItemId    string //
	CreatedAt string //
}

// exampleTaskReceiptsColumns holds the columns for table example_task_receipts.
var exampleTaskReceiptsColumns = ExampleTaskReceiptsColumns{
	TaskKey:   "task_key",
	ItemId:    "item_id",
	CreatedAt: "created_at",
}

// NewExampleTaskReceiptsDao creates and returns a new DAO object for table data access.
func NewExampleTaskReceiptsDao() *ExampleTaskReceiptsDao {
	return &ExampleTaskReceiptsDao{
		group:   "default",
		table:   "example_task_receipts",
		columns: exampleTaskReceiptsColumns,
	}
}

// DB retrieves and returns the underlying raw database management object of current DAO.
func (dao *ExampleTaskReceiptsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of current dao.
func (dao *ExampleTaskReceiptsDao) Table() string {
	return dao.table
}

// Columns returns all column names of current dao.
func (dao *ExampleTaskReceiptsDao) Columns() ExampleTaskReceiptsColumns {
	return dao.columns
}

// Group returns the configuration group name of database of current dao.
func (dao *ExampleTaskReceiptsDao) Group() string {
	return dao.group
}

// Ctx creates and returns the Model for current DAO, It automatically sets the context for current operation.
func (dao *ExampleTaskReceiptsDao) Ctx(ctx context.Context) *gdb.Model {
	return dao.DB().Model(dao.table).Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rollbacks the transaction and returns the error from function f if it returns non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note that, you should not Commit or Rollback the transaction in function f
// as it is automatically handled by this function.
func (dao *ExampleTaskReceiptsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
