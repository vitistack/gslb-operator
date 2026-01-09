package scheduler

import (
	"testing"
	"time"
)

func TestNewScheduler(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		interval time.Duration
		want     *Scheduler
	}{
		{
			name:     "empty-scheduler",
			interval: time.Second * 10,
			want: &Scheduler{
				interval:    time.Second * 10,
				maxOffSets:  20,
				nextOffset:  0,
				jitterRange: time.Second,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewScheduler(tt.interval)

			if got.interval != tt.want.interval {
				t.Errorf("expected interval: %v, got: %v", tt.want.interval, got.interval)
			}

			if got.maxOffSets != tt.want.maxOffSets {
				t.Errorf("expected maxOffSets: %v, got: %v", tt.want.maxOffSets, got.maxOffSets)
			}

			if got.nextOffset != tt.want.nextOffset {
				t.Errorf("expected nextOffset: %v, got: %v", tt.want.nextOffset, got.nextOffset)
			}

			if got.jitterRange != tt.want.jitterRange {
				t.Errorf("expected maxOffSets: %v, got: %v", tt.want.maxOffSets, got.maxOffSets)
			}
		})
	}
}
