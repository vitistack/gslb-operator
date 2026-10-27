package rest

const maxErrorBodySize = 64 * 1024

type HTTPError struct {
	err        error
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string {
	return e.err.Error()
}
