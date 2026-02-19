package manager

import (
	"cmp"
	"log/slog"
	"slices"
	"sync"

	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/models/failover"
)

type ServiceGroupMode int

const (
	ActiveActive ServiceGroupMode = iota
	ActivePassive
	//ActiveActivePassive TODO: decide if this is necessary
	ActiveActiveRoundTrip // TODO: When svc does not exist in DC, then smallest roundtrip time wins
)

func (m *ServiceGroupMode) String() string {
	switch *m {
	case ActiveActive:
		return "ActiveActive"
	case ActivePassive:
		return "ActivePassive"
	default:
		return "ActiveActive"
	}
}

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

	// active is the service that currently holds the active role in a group.
	// In ActivePassive this is straightforward.
	// In ActiveActive it is the service that currently has the lowest roundtrip time (not implemented yet)
	active *service.Service

	//last active service in a service group
	lastActive *service.Service

	// should never receive a nil promotion event
	OnPromotion           func(*PromotionEvent)
	prioritizedDatacenter string
	mu                    sync.RWMutex
}

func NewEmptyServiceGroup() *ServiceGroup {
	return &ServiceGroup{
		mode:       ActiveActive,
		Members:    make([]*service.Service, 0),
		active:     nil,
		lastActive: nil,
		mu:         sync.RWMutex{},
	}
}

// returns the active service in ActivePassive mode,
// or returns the first healthy service in ActiveActive if no explicit active is set.
func (sg *ServiceGroup) GetActive() *service.Service {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
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

// returns the first healthy service of the members in the group.
// In other words, the service that SHOULD be active.
// this is true because the members are sorted on priority.
func (sg *ServiceGroup) firstHealthy() *service.Service {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	for _, svc := range sg.Members {
		if svc.IsHealthy() {
			return svc
		}
	}
	return nil
}

func (sg *ServiceGroup) OnServiceHealthChange(changedService *service.Service, healthy bool) {
	sg.mu.Lock()
	oldActive := sg.active
	if oldActive == nil {
		oldActive = sg.lastActive
	}

	switch sg.mode {
	case ActivePassive:
		if !healthy && sg.active.GetID() == changedService.GetID() { // active has gone down!
			sg.lastActive = sg.active
			sg.mu.Unlock()
			sg.OnPromotion(sg.promoteNextHealthy())
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
			sg.mu.Unlock()
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
	sg.mu.Unlock()
}

// This does not take in to account if the registered service has the highest priority
func (sg *ServiceGroup) RegisterService(newService *service.Service) {
	if newService == nil {
		return
	}

	if sg.memberExists(newService) {
		return
	}

	sg.mu.Lock()
	sg.Members = append(sg.Members, newService)
	sg.mu.Unlock()

	sg.Update()
}

func (sg *ServiceGroup) RemoveService(id string) bool {
	sg.mu.Lock()
	members := sg.Members
	sg.mu.Unlock()

	for idx, member := range members {
		if member.GetID() == id {
			sg.mu.Lock()
			sg.Members = utils.RemoveIndexFromSlice(sg.Members, idx)
			sg.mu.Unlock()
			sg.Update()
			break
		}
	}
	return len(sg.Members) == 0
}

func (sg *ServiceGroup) promoteNextHealthy() *PromotionEvent {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	bslog.Debug("promoting next healthy service", slog.Any("oldActive", sg.active))
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
	if !service.IsHealthy() {
		return false
	}

	if sg.active == nil || !sg.active.IsHealthy() { // if active not healthy then all other healthy services are prioritized
		return service.IsHealthy()
	}

	return service.GetPriority() <= sg.active.GetPriority()
}

// Will configure group mode, based on the state of group members (Members).
// If the state of the group deviates from the requirements of its mode, the mode will change
func (sg *ServiceGroup) SetGroupMode() {
	sg.mu.RLock()
	numServices := len(sg.Members)
	if numServices == 0 {
		sg.mode = ActiveActive
		sg.mu.RUnlock()
		return
	}

	// If one service, default to ActiveActive but don't pre-seed active unless healthy
	if numServices == 1 {
		sg.mode = ActiveActive
		if sg.Members[0].IsHealthy() {
			sg.active = sg.Members[0]
		} else {
			sg.active = nil
		}
		sg.mu.RUnlock()
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
	sg.mu.RUnlock()

	switch sg.mode {
	case ActiveActive:
		// If services have different priorities, switch to ActivePassive
		if !allSamePriority {
			sg.mu.Lock()
			sg.mode = ActivePassive
			sg.mu.Unlock()
		}

	case ActivePassive:
		// If all services have same priority, can switch to ActiveActive
		if allSamePriority {
			sg.mu.Lock()
			sg.mode = ActiveActive
			sg.mu.Unlock()
			// if none healthy, leave active nil
		}

	/*
		case ActiveActivePassive:
			// TODO: implement when requirements are defined
			sg.mode = ActiveActive
			sg.active = sg.firstHealthy()
	*/

	default:
		sg.mu.Lock()
		sg.mode = ActiveActive
		sg.mu.Unlock()
	}
	bslog.Debug("servicegroup mode set", slog.Any("mode", sg.mode.String()))
}

func (sg *ServiceGroup) memberExists(member *service.Service) bool {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	return slices.Contains(sg.Members, member)
}

func (sg *ServiceGroup) Failover(fqdn string, failover failover.Failover) error {
	bslog.Warn("un-implemented servicegroup.Failover(...) method")
	/*
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

	*/
	return nil
}

func (sg *ServiceGroup) Update() {
	sg.mu.RLock()
	if len(sg.Members) == 0 { // dont need to do anything, group should be removed!
		sg.mu.RUnlock()
		return
	}
	sg.mu.RUnlock()

	sg.mu.Lock()
	slices.SortFunc(sg.Members, sortMembersFunc)
	sg.mu.Unlock()

	sg.SetGroupMode()
	firstHealthy := sg.firstHealthy() // who should have the active role!
	if firstHealthy != sg.active {
		// trigger promotion because whoever is active should not be active anymore!
		sg.mu.Lock()
		defer sg.mu.Unlock()
		sg.lastActive = sg.active
		sg.active = firstHealthy

		event := &PromotionEvent{
			Service:   firstHealthy.MemberOf,
			OldActive: sg.lastActive,
			NewActive: sg.active,
		}
		sg.OnPromotion(event)
	}
}

// func passed into slices.SortFunc for sorting the groups members
func sortMembersFunc(a, b *service.Service) int {
	aPriority := a.GetPriority()
	bPriority := b.GetPriority()

	if aPriority != bPriority {
		return cmp.Compare(aPriority, bPriority)
	}

	aRoundtrip := a.GetAverageRoundtrip()
	bRoundtrip := b.GetAverageRoundtrip()

	// handle case where no roundtrip time has been recorded
	aHasRoundtrip := aRoundtrip > 0
	bHasRoundtrip := bRoundtrip > 0

	if aHasRoundtrip && bHasRoundtrip {
		return cmp.Compare(aRoundtrip, bRoundtrip)
	} else if aHasRoundtrip && !bHasRoundtrip { // prioritize the one who has recorded data
		return -1
	} else {
		return 1
	}
}
