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

func NewPaginationResult[T any](params *PaginationParams, items []T) Pagination[T] {
	if params.PageSize < 1 {
		params.PageSize = 50
	}
	if params.Page < 1 {
		params.Page = 1
	}

	numItems := len(items)
	numPages := (numItems + params.PageSize - 1) / params.PageSize

	page := Pagination[T]{
		TotalItems: numItems,
		Page:       params.Page,
		NumPages:   numPages,
	}

	if params.Page < numPages {
		next := params.Page + 1
		page.Next = &next
	}
	if params.Page > 1 {
		prev := params.Page - 1
		page.Previous = &prev
	}

	start := (params.Page - 1) * params.PageSize
	end := min(start+params.PageSize, numItems)
	if start > numItems {
		start = numItems
	}
	page.Items = items[start:end]
	page.NumItems = len(page.Items)

	return page
}
