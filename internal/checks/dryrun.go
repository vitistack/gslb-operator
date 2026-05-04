package checks

import (
	"errors"
	"math/rand"
	"time"
)

type DryRun struct{}

func (dr *DryRun) Check() *HealthCheckResult {

	sleepDuration := time.Duration(100+rand.Intn(400)) * time.Millisecond
	time.Sleep(sleepDuration)
	num := rand.Intn(10)
	if num == 0 { // 10% failure when dryrunning
		return &HealthCheckResult{
			err:       errors.New("dry-run fail"),
			Success:   false,
			CheckTime: float64(sleepDuration),
		}
	}
	return &HealthCheckResult{
		err:       nil,
		Success:   true,
		CheckTime: float64(sleepDuration),
	}
}

func (dr *DryRun) Roundtrip() time.Duration {
	return time.Duration(0)
}
