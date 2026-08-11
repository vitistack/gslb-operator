package pagination

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
func NewPaginationParams() *PaginationParams {
	return &PaginationParams{
		Page:     1,
		PageSize: 50,
	}
}

func NewPaginationResult[T any](params *PaginationParams, totalItems int, items []T) Pagination[T] {
if params.PageSize < 1 {
        params.PageSize = 50
    }
    if params.Page < 1 {
        params.Page = 1
    }

    numPages := (totalItems + params.PageSize - 1) / params.PageSize

    page := Pagination[T]{
        TotalItems: totalItems,
        NumItems:   len(items),
        Page:       params.Page,
        NumPages:   numPages,
        Items:      items,
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
