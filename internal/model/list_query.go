package model

// ListQuery is the shared title-search, creation-time and sorting contract used
// by the CRUD example. The query package also validates non-HTTP calls.
type ListQuery struct {
	Page
	Keyword     string `json:"keyword" dc:"标题关键词，% 和 _ 按字面匹配"`
	CreatedFrom string `json:"createdFrom" dc:"创建时间起点，RFC3339，包含端点"`
	CreatedTo   string `json:"createdTo" dc:"创建时间终点，RFC3339，包含端点"`
	SortBy      string `json:"sortBy" d:"id" v:"in:id,title,created_at,updated_at" dc:"排序字段"`
	SortOrder   string `json:"sortOrder" d:"desc" v:"in:asc,desc" dc:"排序方向，同值按 id 稳定排序"`
}
