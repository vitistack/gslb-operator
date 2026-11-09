package pagination

import (
	"fmt"
	"net/url"

	"github.com/vitistack/gslb-operator/pkg/rest/request"
)

type Pagination[T any] struct {
	TotalItems int  `json:"total_items"`
	NumItems   int  `json:"num_items"`
	NumPages   int  `json:"num_pages"`
	Page       int  `json:"page"`
	Next       *int `json:"next,omitempty"`
	Previous   *int `json:"prev,omitempty"`
	Items      []T  `json:"items"`
}

type PaginationParams struct {
	Page     int `param:"page"`
	PageSize int `param:"pageSize"`
}

// returns a populated pagination parameter object with default values
func NewPaginationParams(params url.Values) (*PaginationParams, error) {
	page := &PaginationParams{}
	if err := request.UnmarshalParams(params, page); err != nil {
		return nil, fmt.Errorf("failed to parse pagination params: %w", err)
	}

	return EnsureDefaults(page), nil
}

func EnsureDefaults(params *PaginationParams) *PaginationParams {
	defaults := &PaginationParams{
		Page:     1,
		PageSize: 50,
	}

	if params.Page < 1 {
		params.Page = defaults.Page
	}
	if params.PageSize < 1 {
		params.PageSize = defaults.PageSize
	}

	return params
}

func NewPaginationResult[T any](params *PaginationParams, totalItems int, pageItems []T) Pagination[T] {
	if params.PageSize < 1 {
		params.PageSize = 50
	}
	if params.Page < 1 {
		params.Page = 1
	}

	numPages := (totalItems + params.PageSize - 1) / params.PageSize

	page := Pagination[T]{
		TotalItems: totalItems,
		NumItems:   len(pageItems),
		Page:       params.Page,
		NumPages:   numPages,
		Items:      pageItems,
	}

	if params.Page < numPages {
		next := params.Page + 1
		page.Next = &next
	}
	if params.Page > 1 {
		prev := params.Page - 1
		page.Previous = &prev
	}

	return page
}
