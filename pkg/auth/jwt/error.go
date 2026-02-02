package jwt

import "net/http"

type Error string

type JWTError struct {
	Code int
	Msg  string `json:"msg"`
}

const (
	ErrUnAuthorized = Error("UnAuthorized")
	ErrForbidden    = Error("FORBIDDEN")
)

var (
	Errors = map[Error]*JWTError{
		ErrUnAuthorized: {
			Code: http.StatusUnauthorized,
			Msg:  "UnAuthorized",
		},
		ErrForbidden: {
			Code: http.StatusForbidden,
			Msg:  "Forbidden",
		},
	}
)
