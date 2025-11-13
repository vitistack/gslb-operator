package checks

import "time"

const DEFAULT_TIMEOUT = time.Second * 5

const (
	MIN_CHECK_INTERVAL = time.Second * 5
	MAX_CHECK_INTERVAL = time.Second * 60
)
