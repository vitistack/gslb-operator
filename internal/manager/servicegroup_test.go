package manager

import (
	"errors"
	"log"
	"testing"
	"time"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
)

type Test struct {
	Name string
}

var activeConfig = model.GSLBConfig{
	Fqdn:       "test.example.com",
	Ip:         "192.168.1.1",
	Port:       "80",
	Datacenter: "dc1",
	Interval:   timesutil.Duration(5 * time.Second),
	Priority:   1,
	CheckType:  "TCP-FULL",
}

var passiveConfig = model.GSLBConfig{
	Fqdn:       "test.example.com",
	Ip:         "192.168.1.1",
	Port:       "80",
	Datacenter: "dc2",
	Interval:   timesutil.Duration(5 * time.Second),
	Priority:   2,
	CheckType:  "TCP-FULL",
}

var active *service.Service
var passive *service.Service

func TestMain(m *testing.M) {
	active, _ = service.NewServiceFromGSLBConfig(activeConfig, service.WithDryRunChecks(true))
	passive, _ = service.NewServiceFromGSLBConfig(passiveConfig, service.WithDryRunChecks(true))
	m.Run()
}

func TestServiceGroup_RegisterService(t *testing.T) {
	group := NewEmptyServiceGroup()
	group.OnPromotion = func(pe *PromotionEvent) {
		log.Println("got promotion")
		if pe != nil {
			t.Errorf("should not be getting promotion event in this")
		}
	}
	if group.mode != ActiveActive {
		t.Error("group mode should be ActiveActive by default")
	}
	group.RegisterService(active)

	if group.mode != ActiveActive {
		t.Error("group mode should be ActiveActive when only one service registered")
	}

	group.RegisterService(passive)
	if group.mode != ActivePassive {
		t.Errorf("Expected group mode: %v, but got: %v, after two services with different priorities registered", ActivePassive, group.mode)
	}
	/*
		if group.active != 0 {
			t.Errorf("Expected activeIndex: %v, but got: %v", 0, group.activeIndex)
		}
	*/
}

func TestServiceGroup_OnServiceHealthChange(t *testing.T) {
	group := NewEmptyServiceGroup()

	group.RegisterService(active)
	group.OnPromotion = func(pe *PromotionEvent) {
		log.Println("got promotion event")
		if pe != nil {
			t.Error("promotion event received when active service in ActiveActive is Healthy/UnHealthy")
		}
	}
	active.SetHealthChangeCallback(func(healthy bool) {
		group.OnServiceHealthChange(active, healthy)
	})

	makeServiceHealthy(active)
	makeServiceUnHealthy(active)

	passive.SetHealthChangeCallback(func(healthy bool) {
		group.OnServiceHealthChange(passive, healthy)
	})
	group.RegisterService(passive)

	group.OnPromotion = func(pe *PromotionEvent) {
		log.Println("got promotion event")
		if pe != nil {
			t.Error("got promotion event when active service in ActivePassive is Healthy")
		}
	}
	makeServiceHealthy(active)

	group.OnPromotion = func(pe *PromotionEvent) {
		log.Println("got promotion event")
		if pe != nil {
			t.Error("got promotion event when passive service in ActivePassive is Healthy, when active is already Healthy")
		}
	}
	makeServiceHealthy(passive)

	group.OnPromotion = func(pe *PromotionEvent) {
		log.Println("got promotion event")
		if pe == nil {
			t.Error("should get promotion event when active service is UnHealthy in ActivePassive")
			return
		}
		if pe.NewActive != passive {
			t.Error("passive is not the new active service in promotion event")
		}
		if pe.OldActive != active {
			t.Error("active is not the old active service in promotion event")
		}
	}
	makeServiceUnHealthy(active)

	group.OnPromotion = func(pe *PromotionEvent) {
		log.Println("got promotion event")
		if pe == nil {
			t.Error("should get promotion event when active service is Healthy again in ActivePassive")
			return
		}
		if pe.NewActive != active {
			t.Error("active is not the new active service in promotion event")
		}
		if pe.OldActive != passive {
			t.Error("passive is not the old active service in promotion event")
		}
	}
	makeServiceHealthy(active)

}

func makeServiceHealthy(service *service.Service) {
	for range service.FailureThreshold {
		service.OnSuccess()
	}
}

func makeServiceUnHealthy(service *service.Service) {
	for range service.FailureThreshold {
		service.OnFailure(errors.New("test"))
	}
}

func TestServiceGroup_memberExists(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		// Named input parameters for target function.
		member *service.Service
		want   bool
	}{
		{
			name: "exists",
			member: &service.Service{
				Fqdn:       "test.example.com",
				Datacenter: "JK",
			},
			want: true,
		},
		{
			name:   "does-not-exist",
			member: &service.Service{},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sg := NewEmptyServiceGroup()
			if tt.want {
				sg.RegisterService(tt.member)
			}

			got := sg.memberExists(tt.member)

			if got != tt.want {
				t.Errorf("memberExists() = %v, want %v", got, tt.want)
			}
		})
	}
}
