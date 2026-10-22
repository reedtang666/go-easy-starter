package model

type Page struct {
	Page     int `json:"page" d:"1" v:"min:1"`
	PageSize int `json:"pageSize" d:"20" v:"between:1,100"`
}

func (p Page) Normalized() Page {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
	return p
}

type List[T any] struct {
	List     []T `json:"list"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}
