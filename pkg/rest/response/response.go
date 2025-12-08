package response

import (
	"encoding/json"
	"net/http"
)


func JSON(w http.ResponseWriter, responseCode int, data any) error {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(responseCode)
	return json.NewEncoder(w).Encode(data)
}