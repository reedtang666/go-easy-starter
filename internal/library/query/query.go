// Package query provides the safe list filters used by the example API.
package query

import (
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"github.com/reedtang666/go-easy-starter/internal/model"
)

// Apply returns a cloned model with filters only, plus a safe ORDER BY value.
// Count and List can share the filters; callers add sorting and paging only to
// the List clone. All identifiers come from the allowlist and values are bound.
func Apply(m *gdb.Model, input model.ListQuery) (*gdb.Model, string, error) {
	sortBy := strings.ToLower(strings.TrimSpace(input.SortBy))
	if sortBy == "" {
		sortBy = "id"
	}
	switch sortBy {
	case "id", "title", "created_at", "updated_at":
	default:
		return nil, "", response.Fail(400, "INVALID_LIST_QUERY", "排序字段无效")
	}
	sortOrder := strings.ToUpper(strings.TrimSpace(input.SortOrder))
	if sortOrder == "" {
		sortOrder = "DESC"
	}
	if sortOrder != "ASC" && sortOrder != "DESC" {
		return nil, "", response.Fail(400, "INVALID_LIST_QUERY", "排序方向无效")
	}
	from, err := parseTime(input.CreatedFrom)
	if err != nil {
		return nil, "", err
	}
	to, err := parseTime(input.CreatedTo)
	if err != nil {
		return nil, "", err
	}
	if from != nil && to != nil && to.Before(*from) {
		return nil, "", response.Fail(400, "INVALID_LIST_QUERY", "结束时间不能早于开始时间")
	}

	filtered := m.Clone()
	if keyword := strings.TrimSpace(input.Keyword); keyword != "" {
		// An explicit escape character avoids SQL-string backslash differences.
		literal := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(keyword)
		filtered = filtered.Where("title ILIKE ? ESCAPE '!'", "%"+literal+"%")
	}
	if from != nil {
		filtered = filtered.Where("created_at >= ?", *from)
	}
	if to != nil {
		filtered = filtered.Where("created_at <= ?", *to)
	}
	order := sortBy + " " + sortOrder
	if sortBy != "id" {
		order += ", id " + sortOrder
	}
	return filtered, order, nil
}

func parseTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, response.Fail(400, "INVALID_LIST_QUERY", "时间格式必须为 RFC3339")
	}
	utc := parsed.UTC()
	return &utc, nil
}
