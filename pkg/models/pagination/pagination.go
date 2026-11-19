package pagination

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/vitistack/gslb-operator/pkg/rest/request"
)

type Pagination[T any] struct {
	Count    int     `json:"count"`
	NumItems int     `json:"limit"`
	Offset   int     `json:"offset"`
	Next     *string `json:"next"`
	Previous *string `json:"prev"`
	Items    []T     `json:"results"`
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

func NewPaginationResult[T any](r *http.Request, params *PaginationParams, count int, pageItems []T) Pagination[T] {
	EnsureDefaults(params)

	numPages := (count + params.PageSize - 1) / params.PageSize

	page := Pagination[T]{
		Count:    count,
		NumItems: len(pageItems),
		Offset:   params.Page,
		Items:    pageItems,
	}

	if params.Page < numPages {
		page.Next = pageLink(r, params.Page+1, params.PageSize)
	}
	if params.Page > 1 {
		page.Previous = pageLink(r, params.Page-1, params.PageSize)
	}

	return page
}

func pageLink(r *http.Request, page, pageSize int) *string {
	u := *r.URL
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	q.Set("pageSize", strconv.Itoa(pageSize))
	u.RawQuery = q.Encode()
	u.Scheme = requestScheme(r)
	u.Host = r.Host
	link := u.String()
	return &link
}

func requestScheme(r *http.Request) string {
	// Behind a TLS-terminating proxy r.TLS is nil, so prefer the forwarded proto.
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
		return proto
	}

	if r.TLS != nil {
		return "https"
	}

	return "http"
}
