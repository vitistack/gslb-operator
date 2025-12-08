package response

import (
	"fmt"
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
)

var (
	Errors = map[Error]RestError{
		ErrInvalidInput: {
			Code:  http.StatusBadRequest,
			Title: string(ErrInvalidInput),
		},
		ErrInternalError: {
			Code: http.StatusInternalServerError,
			Title: string(ErrInternalError),
		},
	}
)

func Err(w http.ResponseWriter, err Error, msg string) error {
	respErr, ok := Errors[err]
	if !ok {
		return fmt.Errorf("REST error response not found: %s", err)
	}
	return JSON(w, respErr.Code, respErr)
}
