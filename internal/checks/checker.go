package checks

import "time"

type Checker interface {
	Check() error
	Roundtrip() time.Duration
}
