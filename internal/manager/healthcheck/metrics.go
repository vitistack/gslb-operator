// collects different metrics for health-checks since start of service
package healthcheck

import (
	"sync"
	"sync/atomic"
	"time"
)

// holds the timestamp of healthcheck time
// and the result of that healthcheck
type Recording struct {
	timestamp time.Time
	result    *Result
}

func NewRecording(res *Result) *Recording {
	return &Recording{
		timestamp: time.Now().Add(-res.timeTaken),
		result:    res,
	}
}

type HealthCheckMetricsCounter struct {
	totalChecks   atomic.Int64
	totalSuccess  atomic.Int64
	totalFailure  atomic.Int64
	maxRecordings int
	recordings    []*Recording // health-check timestamps
	mu            sync.RWMutex
}

func NewMetricsCounter(max int) *HealthCheckMetricsCounter {
	return &HealthCheckMetricsCounter{
		totalChecks:   atomic.Int64{},
		totalSuccess:  atomic.Int64{},
		totalFailure:  atomic.Int64{},
		maxRecordings: max,
		recordings:    make([]*Recording, 0, max),
		mu:            sync.RWMutex{},
	}
}

// total checks in the last given time-frame
func (c *HealthCheckMetricsCounter) Last(dur time.Duration) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0

	interval := time.Now().Add(-dur)

	for i := len(c.recordings) - 1; i >= 0; i-- {
		if !c.recordings[i].timestamp.Before(interval) {
			count++
		} else {
			// since we start from the back
			// the first recording that is BEFORE the interval
			// means that every interval next after this one is also before the interval
			// therefore no need to check them when we know they are not going to hit
			break
		}
	}

	return count
}

// total checks that have been successfull in the last given time-frame
func (c *HealthCheckMetricsCounter) SuccessLast(dur time.Duration) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0

	interval := time.Now().Add(-dur)

	for i := len(c.recordings) - 1; i >= 0; i-- {
		if !c.recordings[i].timestamp.Before(interval) && c.recordings[i].result.Success {
			count++
		} else {
			// same reason as Last(...) func
			break
		}
	}

	return count
}

// total checks that have been failure in the last given time-frame
func (c *HealthCheckMetricsCounter) FailuresLast(dur time.Duration) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0

	interval := time.Now().Add(-dur)

	for i := len(c.recordings) - 1; i >= 0; i-- {
		if !c.recordings[i].timestamp.Before(interval) && !c.recordings[i].result.Success {
			count++
		} else {
			// same reason as Last(...) func
			break
		}
	}

	return count
}

func (c *HealthCheckMetricsCounter) Record(result *Result) {
	c.totalChecks.Add(1)

	if result.Success {
		c.totalSuccess.Add(1)
	} else {
		c.totalFailure.Add(1)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.recordings) == c.maxRecordings {
		c.recordings = append(c.recordings[1:], NewRecording(result))
	} else {
		c.recordings = append(c.recordings, NewRecording(result))
	}
}
