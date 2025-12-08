package request

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
