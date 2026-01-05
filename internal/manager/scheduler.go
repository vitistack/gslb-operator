package manager

import (
	"time"

	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
)

// wrapper for scheduling services' ticker and quit
// TODO: sub-interval scheduling, e.g. wait 1s then schedule a new task on new timer
type scheduler struct {
	interval timesutil.Duration
	ticker   *time.Ticker
	quit     chan struct{}
	//OnTick   func()
}

func newScheduler(duration timesutil.Duration) *scheduler {
	return &scheduler{
		interval: duration,
		ticker:   time.NewTicker(time.Duration(duration)),
		quit:     make(chan struct{}),
	}
}

func (s *scheduler) Stop() {
	s.ticker.Stop()
}

func (s *scheduler) Start() {
	s.ticker.Reset(time.Duration(s.interval))
}
