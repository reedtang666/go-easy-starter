package query

import (
	"context"
	"errors"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"github.com/reedtang666/go-easy-starter/internal/model"
)

func queryModel(t *testing.T) *gdb.Model {
	t.Helper()
	db, err := gdb.New(gdb.ConfigNode{
		Type: "pgsql", Link: "pgsql:host=127.0.0.1 port=1 user=unused dbname=unused sslmode=disable",
		TimeMaintainDisabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(context.Background()) })
	return db.Model("example_items").Safe().Unscoped().Where("owner_id", 42)
}

func sqlFor(t *testing.T, m *gdb.Model, count bool) string {
	t.Helper()
	sql, err := gdb.ToSQL(context.Background(), func(ctx context.Context) error {
		if count {
			_, err := m.Ctx(ctx).Count()
			return err
		}
		_, err := m.Ctx(ctx).All()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return sql
}

func TestApplySorting(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input model.ListQuery
		want  string
	}{
		{name: "defaults", want: "id DESC"},
		{name: "title ascending", input: model.ListQuery{SortBy: "title", SortOrder: "asc"}, want: "title ASC, id ASC"},
		{name: "created time descending", input: model.ListQuery{SortBy: "created_at"}, want: "created_at DESC, id DESC"},
		{name: "updated time", input: model.ListQuery{SortBy: "updated_at", SortOrder: "asc"}, want: "updated_at ASC, id ASC"},
		{name: "normalized", input: model.ListQuery{SortBy: " TITLE ", SortOrder: " ASC "}, want: "title ASC, id ASC"},
		{name: "primary key ascending", input: model.ListQuery{SortBy: "id", SortOrder: "asc"}, want: "id ASC"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filtered, order, err := Apply(queryModel(t), tc.input)
			if err != nil || filtered == nil || order != tc.want {
				t.Fatalf("order=%q model=%v err=%v; want=%q", order, filtered, err, tc.want)
			}
		})
	}
}

func TestApplyRejectsInvalidQueries(t *testing.T) {
	for _, input := range []model.ListQuery{
		{SortBy: "id; DROP TABLE users"},
		{SortBy: "owner_id"},
		{SortOrder: "desc NULLS FIRST"},
		{CreatedFrom: "2026-10-22"},
		{CreatedTo: "invalid"},
		{CreatedFrom: "2026-10-22T12:00:00Z", CreatedTo: "2026-10-22T11:59:59Z"},
	} {
		_, _, err := Apply(queryModel(t), input)
		var validation *response.Error
		if !errors.As(err, &validation) || validation.Status != 400 {
			t.Fatalf("input=%+v error=%v; want HTTP 400", input, err)
		}
	}
}

func TestApplyFiltersRemainOnCountAndList(t *testing.T) {
	base := queryModel(t)
	filtered, order, err := Apply(base, model.ListQuery{
		Page:    model.Page{Page: 2, PageSize: 1},
		Keyword: "50%_! O'Reilly", CreatedFrom: "2026-10-22T10:00:00+08:00", CreatedTo: "2026-10-22T02:00:00Z",
		SortBy: "title", SortOrder: "asc",
	})
	if err != nil {
		t.Fatal(err)
	}
	count := sqlFor(t, filtered.Clone(), true)
	list := sqlFor(t, filtered.Clone().Order(order).Page(2, 1), false)
	for name, sql := range map[string]string{"count": count, "list": list} {
		for _, want := range []string{`"owner_id"=42`, "title ILIKE", "50!%!_!! O", "ESCAPE '!'", "created_at >=", "created_at <=", "2026-10-22 02:00:00"} {
			if !strings.Contains(sql, want) {
				t.Errorf("%s missing %q: %s", name, want, sql)
			}
		}
	}
	if strings.Contains(count, "ORDER BY") || strings.Contains(count, "LIMIT") || strings.Contains(count, "OFFSET") {
		t.Fatalf("Count inherited sorting or pagination: %s", count)
	}
	if !strings.Contains(list, `ORDER BY "title" ASC,"id" ASC`) || !strings.Contains(list, "LIMIT 1 OFFSET 1") {
		t.Fatalf("list missing stable order/pagination: %s", list)
	}
	if sql := sqlFor(t, base.Clone(), false); strings.Contains(sql, "ILIKE") || strings.Contains(sql, "created_at") {
		t.Fatalf("Apply mutated caller's base model: %s", sql)
	}
}

func TestApplyAcceptsOpenTimeBoundsAndEmptyKeyword(t *testing.T) {
	for _, input := range []model.ListQuery{
		{CreatedFrom: "2026-10-22T00:00:00Z"},
		{CreatedTo: "2026-10-22T00:00:00Z"},
		{Keyword: "  "},
	} {
		filtered, _, err := Apply(queryModel(t), input)
		if err != nil {
			t.Fatal(err)
		}
		sql := sqlFor(t, filtered, false)
		if strings.Contains(sql, "ILIKE") {
			t.Fatalf("empty keyword added search: %s", sql)
		}
	}
}

func TestApplyDoesNotTreatValidZeroTimeAsMissing(t *testing.T) {
	filtered, _, err := Apply(queryModel(t), model.ListQuery{CreatedFrom: "0001-01-01T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if sql := sqlFor(t, filtered, false); !strings.Contains(sql, "created_at >=") {
		t.Fatalf("valid timestamp was dropped: %s", sql)
	}
}
