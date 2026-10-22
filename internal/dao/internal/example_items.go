// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ExampleItemsDao is the data access object for table example_items.
type ExampleItemsDao struct {
	table   string              // table is the underlying table name of the DAO.
	group   string              // group is the database configuration group name of current DAO.
	columns ExampleItemsColumns // columns contains all the column names of Table for convenient usage.
}

// ExampleItemsColumns defines and stores column names for table example_items.
type ExampleItemsColumns struct {
	Id        string //
	OwnerId   string //
	Title     string //
	Content   string //
	TaskRuns  string //
	CreatedAt string //
	UpdatedAt string //
}

// exampleItemsColumns holds the columns for table example_items.
var exampleItemsColumns = ExampleItemsColumns{
	Id:        "id",
	OwnerId:   "owner_id",
	Title:     "title",
	Content:   "content",
	TaskRuns:  "task_runs",
	CreatedAt: "created_at",
	UpdatedAt: "updated_at",
}

// NewExampleItemsDao creates and returns a new DAO object for table data access.
func NewExampleItemsDao() *ExampleItemsDao {
	return &ExampleItemsDao{
		group:   "default",
		table:   "example_items",
		columns: exampleItemsColumns,
	}
}

// DB retrieves and returns the underlying raw database management object of current DAO.
func (dao *ExampleItemsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of current dao.
func (dao *ExampleItemsDao) Table() string {
	return dao.table
}

// Columns returns all column names of current dao.
func (dao *ExampleItemsDao) Columns() ExampleItemsColumns {
	return dao.columns
}

// Group returns the configuration group name of database of current dao.
func (dao *ExampleItemsDao) Group() string {
	return dao.group
}

// Ctx creates and returns the Model for current DAO, It automatically sets the context for current operation.
func (dao *ExampleItemsDao) Ctx(ctx context.Context) *gdb.Model {
	return dao.DB().Model(dao.table).Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rollbacks the transaction and returns the error from function f if it returns non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note that, you should not Commit or Rollback the transaction in function f
// as it is automatically handled by this function.
func (dao *ExampleItemsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
