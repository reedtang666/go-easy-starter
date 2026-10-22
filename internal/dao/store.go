package dao

import (
	"context"
	"errors"
	"fmt"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/lib/pq"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"sort"
	"strings"
)

type Store struct{ DB gdb.DB }

func New(db gdb.DB) *Store { return &Store{DB: db} }
func (s *Store) Users(ctx context.Context) *gdb.Model {
	return s.DB.Model(Users.Table()).Ctx(ctx).Safe()
}
func (s *Store) Files(ctx context.Context) *gdb.Model {
	return s.DB.Model(Attachments.Table()).Ctx(ctx).Safe()
}
func (s *Store) Items(ctx context.Context) *gdb.Model {
	return s.DB.Model(ExampleItems.Table()).Ctx(ctx).Safe()
}
func Error(err error) error {
	if err == nil {
		return nil
	}
	var p *pq.Error
	if errors.As(err, &p) {
		switch p.Code {
		case "23505":
			return response.Fail(409, "ALREADY_EXISTS", "记录已存在")
		case "23503":
			return response.Fail(409, "RESOURCE_IN_USE", "资源仍被引用")
		}
	}
	return err
}

// InsertID uses PostgreSQL RETURNING explicitly. Older GoFrame metadata lookup
// can mistake PostgreSQL 18 NOT NULL constraints for the primary-key entry.
func (s *Store) InsertID(ctx context.Context, table string, data g.Map) (int64, error) {
	columns := make([]string, 0, len(data))
	for column := range data {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	names := make([]string, 0, len(columns))
	marks := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns))
	for _, column := range columns {
		names = append(names, pq.QuoteIdentifier(column))
		marks = append(marks, "?")
		args = append(args, data[column])
	}
	value, e := s.DB.GetValue(ctx, fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING id", pq.QuoteIdentifier(table), strings.Join(names, ","), strings.Join(marks, ",")), args...)
	if e != nil {
		return 0, Error(e)
	}
	return value.Int64(), nil
}
