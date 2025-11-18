package manager

import (
	"time"

	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
)

// wrapper for scheduling services' ticker and quit
type scheduler struct {
	interval timesutil.Duration
	ticker   *time.Ticker
	quit     chan struct{}
}

func newScheduler(duration timesutil.Duration) *scheduler {
	return &scheduler{
		interval: duration,
		ticker: time.NewTicker(time.Duration(duration)),
		quit:   make(chan struct{}),
	}
}

func (s *scheduler) Stop() {
	s.ticker.Stop()
}

func (s *scheduler) Start() {
	s.ticker.Reset(time.Duration(s.interval))
}