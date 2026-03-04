package response

import (
	"net/http"
)

type Error string

type RestError struct {
	Code    int    `json:"code"`
	Title   string `json:"title"`
	Details string `json:"details"`
}

const (
	ErrInvalidInput  = Error("INVALID_INPUT")
	ErrInternalError = Error("INTERNAL_ERROR")
	ErrNotFound      = Error("NOT_FOUND")
)

var (
	Errors = map[Error]RestError{
		ErrInvalidInput: {
			Code:  http.StatusBadRequest,
			Title: string(ErrInvalidInput),
		},
		ErrInternalError: {
			Code:  http.StatusInternalServerError,
			Title: string(ErrInternalError),
		},
		ErrNotFound: {
			Code:  http.StatusNotFound,
			Title: string(ErrNotFound),
		},
	}
)
