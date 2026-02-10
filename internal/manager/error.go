package manager

import "errors"

var (
	ErrCannotPromoteUnHealthyService = errors.New("cannot promote UnHealthy service")
	ErrServiceNotFound               = errors.New("service not found")
	ErrServiceNotFoundInGroup        = errors.New("service not found in service group")
)
