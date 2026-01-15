package service

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidGslbConfig   = errors.New("invalid GSLB - config")
	ErrEmptyServiceId      = fmt.Errorf("%w: empty service id", ErrInvalidGslbConfig)
	ErrUnableToParseIpAddr = fmt.Errorf("%w: unable to parse ip address", ErrInvalidGslbConfig)
	ErrUnableToResolveAddr = fmt.Errorf("%w: unable to resolve address", ErrInvalidGslbConfig)
)
