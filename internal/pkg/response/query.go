package response

import "strings"

type PaginationQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

func (q *PaginationQuery) Normalize(defaultPageSize int) {
	if q.Page < 1 {
		q.Page = 1
	}

	if q.PageSize < 1 {
		q.PageSize = defaultPageSize
	}
}

type SortQuery struct {
	SortBy    string `form:"sort_by"`
	SortOrder string `form:"sort_order"`
}

func (q *SortQuery) Normalize(defaultSortBy, defaultSortOrder string) {
	q.SortBy = strings.TrimSpace(q.SortBy)
	q.SortOrder = strings.TrimSpace(q.SortOrder)

	if q.SortBy == "" {
		q.SortBy = defaultSortBy
	}

	if q.SortOrder == "" {
		q.SortOrder = defaultSortOrder
		return
	}

	q.SortOrder = strings.ToUpper(q.SortOrder)
	if q.SortOrder != "ASC" && q.SortOrder != "DESC" {
		q.SortOrder = defaultSortOrder
	}
}

type FilterQuery struct {
	PaginationQuery
	SortQuery
}

func (q *FilterQuery) Normalize(defaultPageSize int, defaultSortBy, defaultSortOrder string) {
	q.PaginationQuery.Normalize(defaultPageSize)
	q.SortQuery.Normalize(defaultSortBy, defaultSortOrder)
}
