package pool

import "errors"

var ErrPutOnClosedPool = errors.New("cannot accept job on closed pool")
