package response

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func JSON(w http.ResponseWriter, responseCode int, data any) error {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(responseCode)
	return json.NewEncoder(w).Encode(data)
}

func Err(w http.ResponseWriter, err Error, msg string) error {
	respErr, ok := Errors[err]
	if !ok {
		return fmt.Errorf("REST error response not found: %s", err)
	}
	respErr.Details = msg
	return JSON(w, respErr.Code, respErr)
}
