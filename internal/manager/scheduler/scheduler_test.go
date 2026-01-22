package scheduler

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"go.uber.org/zap"
)

var genericGSLBConfig = model.GSLBConfig{
	Ip:         "192.168.1.1",
	Port:       "80",
	Datacenter: "dc1",
	Interval:   timesutil.Duration(5 * time.Second),
	Priority:   1,
	CheckType:       "TCP-FULL",
}

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

func TestScheduler_Loop(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		interval time.Duration
	}{
		{
			name:     "100-services-on-5s",
			interval: time.Second * 5,
		},
		{
			name:     "100-services-on-15s",
			interval: time.Second * 15,
		},
		{
			name:     "100-services-on-45s",
			interval: time.Second * 45,
		},
		{
			name:     "100-services-on-60s",
			interval: time.Second * 60,
		},
	}

	numServices := 100
	urls := randomUrlIDs(numServices)

	services := make([]*service.Service, 0, 100)
	logger, _ := zap.NewDevelopment()

	for idx := range numServices {
		genericGSLBConfig.Fqdn = urls[idx]
		svc, _ := service.NewServiceFromGSLBConfig(genericGSLBConfig, logger.Sugar(), true)
		services = append(services, svc)
	}

	for _, tt := range tests {
		scheduler := NewScheduler(tt.interval)
		scheduler.OnTick = func(s *service.Service) {
			t.Logf("received tick for: %s\n", s.Fqdn)
		}

		for _, svc := range services {
			scheduler.ScheduleService(svc)
		}
		time.Sleep(time.Second * 6)
	}
}

func randomUrlIDs(num int) []string {
	baseUrl := "test.example.com"
	urls := make([]string, 0, num)

	const charSet = "abcdefghijklmnopqrstuvwxyz"
	for range num {
		idx := rand.Intn(len(charSet))
		urls = append(urls, fmt.Sprintf("%v/%v", baseUrl, charSet[idx]))
	}

	return urls
}
