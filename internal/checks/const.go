package checks

import "time"

const DEFAULT_TIMEOUT = time.Millisecond * 500

const (
	MIN_CHECK_INTERVAL = time.Second * 5
	MAX_CHECK_INTERVAL = time.Second * 60
)

const (
	HTTP     = "HTTP"
	HTTPS    = "HTTPS"
	TCP_FULL = "TCP-FULL"
	TCP_HALF = "TCP-HALF"
)
