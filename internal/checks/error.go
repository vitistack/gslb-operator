package checks

import "errors"

var ErrServiceUnavailable = errors.New("connect: connection refused")
