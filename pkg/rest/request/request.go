package request

import (
	"encoding/json"
	"io"
)

func JSONDECODE[T any](body io.Reader, dest *T) error {
	return json.NewDecoder(body).Decode(dest)
}
