package pagination

type Pagination struct {
	TotalItems int  `json:"total_items"`
	NumItems   int  `json:"num_items"`
	NumPages   int  `json:"num_pages"`
	Page       int  `json:"page"`
	Next       *int `json:"next,omitempty"`
	Previous   *int `json:"prev,omitempty"`
	//Query	string TODO: Maybe this would be cool???? or have a custom query object???
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
