package example

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/reedtang666/go-easy-starter/internal/dao"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"github.com/reedtang666/go-easy-starter/internal/model"
	view "github.com/reedtang666/go-easy-starter/internal/model/view"
	"time"
)

type sExample struct{ store *dao.Store }

func New(s *dao.Store) *sExample { return &sExample{store: s} }
func (s *sExample) Get(ctx context.Context, owner, id int64) (*view.ExampleItem, error) {
	var item *view.ExampleItem
	e := s.store.Items(ctx).Where("id", id).Where("owner_id", owner).Scan(&item)
	if e != nil {
		return nil, e
	}
	if item == nil {
		return nil, response.Fail(404, "ITEM_NOT_FOUND", "记录不存在")
	}
	return item, nil
}
func (s *sExample) List(ctx context.Context, owner int64, p model.Page) (*model.List[*view.ExampleItem], error) {
	p = p.Normalized()
	m := s.store.Items(ctx).Where("owner_id", owner)
	total, e := m.Clone().Count()
	if e != nil {
		return nil, e
	}
	list := []*view.ExampleItem{}
	e = m.Clone().OrderDesc("id").Page(p.Page, p.PageSize).Scan(&list)
	return &model.List[*view.ExampleItem]{List: list, Total: total, Page: p.Page, PageSize: p.PageSize}, e
}
func (s *sExample) Create(ctx context.Context, owner int64, title, content string) (*view.ExampleItem, error) {
	id, e := s.store.InsertID(ctx, dao.ExampleItems.Table(), g.Map{"owner_id": owner, "title": title, "content": content})
	if e != nil {
		return nil, e
	}
	return s.Get(ctx, owner, id)
}
func (s *sExample) Update(ctx context.Context, owner, id int64, title, content string) (*view.ExampleItem, error) {
	if _, e := s.Get(ctx, owner, id); e != nil {
		return nil, e
	}
	_, e := s.store.Items(ctx).Where("id", id).Where("owner_id", owner).Data(g.Map{"title": title, "content": content, "updated_at": time.Now()}).Update()
	if e != nil {
		return nil, e
	}
	return s.Get(ctx, owner, id)
}
func (s *sExample) Delete(ctx context.Context, owner, id int64) error {
	if _, e := s.Get(ctx, owner, id); e != nil {
		return e
	}
	_, e := s.store.Items(ctx).Where("id", id).Where("owner_id", owner).Delete()
	return e
}
func (s *sExample) ApplyTask(ctx context.Context, owner, id int64, key string) error {
	return s.store.DB.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var item *view.ExampleItem
		if e := tx.Model("example_items").Ctx(ctx).Where("id", id).Where("owner_id", owner).LockUpdate().Scan(&item); e != nil {
			return e
		}
		if item == nil {
			return response.Fail(404, "ITEM_NOT_FOUND", "记录不存在")
		}
		result, e := tx.Exec("INSERT INTO example_task_receipts(task_key,item_id) VALUES(?,?) ON CONFLICT(task_key) DO NOTHING", key, id)
		if e != nil {
			return e
		}
		n, e := result.RowsAffected()
		if e != nil || n == 0 {
			return e
		}
		_, e = tx.Model("example_items").Ctx(ctx).Where("id", id).Data(g.Map{"task_runs": gdb.Raw("task_runs + 1"), "updated_at": time.Now()}).Update()
		return e
	})
}
