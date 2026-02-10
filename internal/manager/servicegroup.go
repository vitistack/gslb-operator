package manager

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils"
	"github.com/vitistack/gslb-operator/pkg/models/failover"
)

type ServiceGroupMode int

const (
	ActiveActive ServiceGroupMode = iota
	ActivePassive
	//ActiveActivePassive TODO: decide if this is necessary
	ActiveActiveRoundTrip // TODO: When svc does not exist in DC, then smallest roundtrip time wins
)

// PromotionEvent is an event that occurs when there is a new Active service in a service group.
// It is triggered using the OnPromotion function of the ServiceGroup belonging to that service.
// The new active service is always healthy, unless no services are healthy in the service group. Then the active service is nil in the event.
type PromotionEvent struct {
	Service   string
	NewActive *service.Service
	OldActive *service.Service
}

type ServiceGroup struct {
	mode ServiceGroupMode

	// sorted by priority.
	// if two services have the same priority, then the prioritizedDatacenter will decide who gets sorted into what index.
	Members []*service.Service

	// See modes in comments below
	active *service.Service

	//last active service in a service group
	lastActive *service.Service

	// should never receive a nil promotion event
	OnPromotion           func(*PromotionEvent)
	prioritizedDatacenter string
}

func NewEmptyServiceGroup() *ServiceGroup {
	datacenter := config.GetInstance().Server().Datacenter()
	return &ServiceGroup{
		mode:                  ActiveActive,
		Members:               make([]*service.Service, 0),
		active:                nil,
		lastActive:            nil,
		prioritizedDatacenter: datacenter,
	}
}

// returns the active service in ActivePassive mode,
// or returns the first healthy service in ActiveActive if no explicit active is set.
func (sg *ServiceGroup) GetActive() *service.Service {
	switch sg.mode {
	case ActivePassive:
		if sg.active != nil {
			return sg.active
		}
	default:
		for _, svc := range sg.Members {
			if svc.IsHealthy() {
				return svc
			}
		}
	}

	return sg.active
}

func (sg *ServiceGroup) firstHealthy() *service.Service {
	for _, svc := range sg.Members {
		if svc.IsHealthy() {
			return svc
		}
	}
	return nil
}

func (sg *ServiceGroup) OnServiceHealthChange(changedService *service.Service, healthy bool) {
	oldActive := sg.active
	if oldActive == nil {
		oldActive = sg.lastActive
	}
	switch sg.mode {
	case ActivePassive:
		if !healthy && sg.active.GetID() == changedService.GetID() { // active has gone down!
			sg.lastActive = sg.active
			sg.OnPromotion(sg.promoteNext())
			return
		}

		if healthy && sg.triggerPromotion(changedService) {
			event := &PromotionEvent{
				Service:   changedService.Fqdn,
				OldActive: oldActive,
				NewActive: changedService,
			}

			sg.lastActive = sg.active
			sg.active = changedService
			sg.OnPromotion(event)
		}

	case ActiveActive:
		if healthy {
			// If prioritized DC service becomes healthy, it must become active (single DNS record).
			if changedService.Datacenter == sg.prioritizedDatacenter && changedService != sg.active {
				sg.OnPromotion(&PromotionEvent{
					Service:   changedService.Fqdn,
					NewActive: changedService,
					OldActive: sg.active,
				})
				sg.active = changedService
				return
			}
			// If there is no active or the current active is unhealthy, promote this healthy service.
			if sg.active == nil || !sg.active.IsHealthy() {
				sg.OnPromotion(&PromotionEvent{
					Service:   changedService.Fqdn,
					NewActive: changedService,
					OldActive: sg.active,
				})
				sg.active = changedService
				return
			}
			return
		}

		// unhealthy
		if changedService.GetID() == sg.active.GetID() {
			next := sg.firstHealthy()
			if next != nil {
				sg.OnPromotion(&PromotionEvent{
					Service:   changedService.Fqdn,
					NewActive: next,
					OldActive: sg.active,
				})
				sg.lastActive = sg.active
				sg.active = next
				return
			}

			// all down -> signal DNS delete (single-record)
			sg.OnPromotion(&PromotionEvent{
				Service:   changedService.Fqdn,
				NewActive: nil,
				OldActive: sg.active,
			})
			sg.lastActive = sg.active
			sg.active = nil
		}
	}
}

// This does not take in to account if the registered service has the highest priority
// because a registered service is NEVER healthy at first
func (sg *ServiceGroup) RegisterService(newService *service.Service) {
	if newService == nil {
		return
	}

	if sg.memberExists(newService) {
		return
	}

	sg.Members = append(sg.Members, newService)
	slices.SortFunc(sg.Members, func(a, b *service.Service) int {
		aPriority := a.GetPriority()
		bPriority := b.GetPriority()

		if aPriority != bPriority {
			return cmp.Compare(aPriority, bPriority)
		}

		// equal priority - prioritized datacenter decides (ActiveActive tie-break)
		if a.Datacenter == sg.prioritizedDatacenter {
			return -1
		} else if b.Datacenter == sg.prioritizedDatacenter {
			return 1
		}
		return 0
	})

	sg.SetGroupMode()
}

func (sg *ServiceGroup) RemoveService(rmService *service.Service) bool {
	for idx, member := range sg.Members {
		if member.GetID() == rmService.GetID() {
			sg.Members = utils.RemoveIndexFromSlice(sg.Members, idx)
			if member.GetID() == sg.active.GetID() {
				sg.OnPromotion(sg.promoteNext())
			}
			sg.SetGroupMode()
			break
		}
	}
	return len(sg.Members) == 0
}

func (sg *ServiceGroup) promoteNext() *PromotionEvent {
	oldActive := sg.active

	// Try to find next healthy service with highest priority (lowest priority number)
	bestIdx := -1
	bestPriority := int(^uint(0) >> 1) // max int

	for i, svc := range sg.Members {
		if svc.IsHealthy() && svc.GetPriority() < bestPriority {
			bestIdx = i
			bestPriority = svc.GetPriority()
		}
	}

	if bestIdx != -1 {
		sg.active = sg.Members[bestIdx]
		return &PromotionEvent{
			Service:   oldActive.Fqdn,
			NewActive: sg.active,
			OldActive: oldActive,
		}
	}

	// No healthy services: signal DNS delete (NewActive=nil)
	sg.active = nil
	return &PromotionEvent{
		Service:   oldActive.Fqdn,
		NewActive: nil,
		OldActive: oldActive,
	}
}

func (sg *ServiceGroup) triggerPromotion(service *service.Service) bool {
	if sg.active == nil || !sg.active.IsHealthy() { // if active not healthy then all other healthy services are prioritized
		return service.IsHealthy()
	}

	if !service.IsHealthy() {
		return false
	}

	return service.GetPriority() <= sg.active.GetPriority()
}

// Will configure group mode, based on the state of group members (Members).
// If the state of the group deviates from the requirements of its mode, the mode will change
func (sg *ServiceGroup) SetGroupMode() {
	numServices := len(sg.Members)

	// If one service, default to ActiveActive but don't pre-seed active unless healthy
	if numServices == 1 {
		sg.mode = ActiveActive
		if sg.Members[0].IsHealthy() {
			sg.active = sg.Members[0]
		} else {
			sg.active = nil
		}
		return
	}

	// Check if all services have the same priority (ActiveActive requirement)
	allSamePriority := true
	firstPriority := sg.Members[0].GetPriority()
	for _, svc := range sg.Members[1:] {
		if svc.GetPriority() != firstPriority {
			allSamePriority = false
			break
		}
	}

	switch sg.mode {
	case ActiveActive:
		// If services have different priorities, switch to ActivePassive
		if !allSamePriority {
			sg.mode = ActivePassive
			sg.active = sg.firstHealthy()
			// do not seed fake active if none healthy
		}

	case ActivePassive:
		// If all services have same priority, can switch to ActiveActive
		if allSamePriority {
			sg.mode = ActiveActive
			sg.active = sg.firstHealthy()
			// if none healthy, leave active nil (single-record: DNS should be absent)
		}

	/*
		case ActiveActivePassive:
			// TODO: implement when requirements are defined
			sg.mode = ActiveActive
			sg.active = sg.firstHealthy()
	*/

	default:
		sg.mode = ActiveActive
		sg.active = sg.firstHealthy()
	}
}

func (sg *ServiceGroup) memberExists(member *service.Service) bool {
	return slices.Contains(sg.Members, member)
}

func (sg *ServiceGroup) Failover(fqdn string, failover failover.Failover) error {
	var failoverSvc *service.Service
	for _, svc := range sg.Members {
		if svc.Datacenter == failover.Datacenter {
			failoverSvc = svc
			break
		}
	}

	if failoverSvc == nil {
		return fmt.Errorf("no service in service group registered with datacenter: %s", failover.Datacenter)
	}

	if !failoverSvc.IsHealthy() {
		return fmt.Errorf("%w: service not considered healthy: %v", ErrCannotPromoteUnHealthyService, failoverSvc)
	}

	sg.lastActive = sg.active
	sg.active = failoverSvc
	//TODO: is this enough?
	sg.OnPromotion(&PromotionEvent{
		Service:   fqdn,
		NewActive: failoverSvc,
		OldActive: sg.lastActive,
	})

	return nil
}
