package checks

import (
	"sync"
	"time"
)

type RoundTripper struct {
	mu                sync.RWMutex
	currentTripStart  time.Time
	roundtrips        []time.Duration
	roundtripIdx      int // current index to populate
	count             int
	roundtripCapacity int
}

func NewRoundtripper() *RoundTripper {
	return &RoundTripper{
		mu:                sync.RWMutex{},
		roundtrips:        make([]time.Duration, 20),
		roundtripIdx:      0,
		count:             0,
		roundtripCapacity: 20,
	}
}

func (rt *RoundTripper) startRecording() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.currentTripStart = time.Now()
}

func (rt *RoundTripper) endRecording() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.roundtrips[rt.roundtripIdx] = time.Since(rt.currentTripStart)
	rt.roundtripIdx = (rt.roundtripIdx + 1) % rt.roundtripCapacity

	if rt.count < rt.roundtripCapacity {
		rt.count++
	}
}

func (rt *RoundTripper) AverageRoundtripTime() time.Duration {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	if rt.count == 0 {
		return time.Duration(0)
	}

	var sum time.Duration

	for _, trip := range rt.roundtrips {
		sum += trip
	}

	return sum / time.Duration(rt.count)
}
