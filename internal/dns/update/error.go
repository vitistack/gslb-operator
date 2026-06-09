package update

import "fmt"

type UpdateError struct {
	err    error
	server string
}

func (e UpdateError) Error() string {
	return fmt.Errorf("%s: %w", e.server, e.err).Error()
}
