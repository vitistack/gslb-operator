package checks

import (
	"errors"
	"math/rand"
	"time"
)

type DryRun struct{}

func (dr *DryRun) Check() error {

	sleepDuration := time.Duration(100+rand.Intn(400)) * time.Millisecond
	time.Sleep(sleepDuration)
	num := rand.Intn(10)
	if num == 0 { // 10% failure when dryrunning
		return errors.New("dry-run fail")
	}
	return nil
}

func (dr *DryRun) Roundtrip() time.Duration {
	return time.Duration(0)
}
