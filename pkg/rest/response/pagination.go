package response

type Pagination struct {
	TotalItems int  `json:"total_items"`
	NumItems   int  `json:"num_items"`
	NumPages   int  `json:"num_pages"`
	Page       int  `json:"page"`
	Next       *int `json:"next,omitempty"`
	Previous   *int `json:"prev,omitempty"`
	//Query	string TODO: Maybe this would be cool???? or have a custom query object???
}
