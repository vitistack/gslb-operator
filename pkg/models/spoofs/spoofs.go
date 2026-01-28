package spoofs

import (
	"github.com/vitistack/gslb-operator/pkg/models/pagination"
)

type Spoof struct {
	FQDN string `json:"fqdn"`
	IP   string `json:"ip"`
	DC   string `json:"datacenter"`
}

type SpoofResponse struct {
	pagination.Pagination
	Items []Spoof `json:"items"`
}

func NewSpoofResponse(items []Spoof, params *pagination.PaginationParams) *SpoofResponse {
	TotalItems := len(items)
	numPages := TotalItems/params.PageSize + 1

	resp := &SpoofResponse{
		Pagination: pagination.Pagination{
			TotalItems: TotalItems,
			NumPages:   numPages,
			Page:       params.Page,
		},
		Items: items, // TODO: Actually handle the paginated items properly
	}

	if params.Page < numPages {
		*resp.Next = params.Page + 1
	}
	if params.Page > 1 {
		*resp.Previous = params.Page - 1
	}

	return resp
}
