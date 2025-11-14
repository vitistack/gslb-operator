package manager

import "errors"

var ErrServiceNotFound = errors.New("service not found")

var (
	ErrCannotPromoteUnHealthyService = errors.New("cannot promote UnHealthy service")
	ErrServiceNotFoundInGroup        = errors.New("service not found in service group")
)
