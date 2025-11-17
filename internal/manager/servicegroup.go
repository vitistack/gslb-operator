package manager

import (
	"cmp"
	"slices"

	"github.com/vitistack/gslb-operator/internal/service"
)

type ServiceGroupMode int

const (
	ActiveActive ServiceGroupMode = iota
	ActivePassive
	ActiveActivePassive // TODO: decide if this is necessary
)

type PromotionEvent struct {
	Service   string
	NewActive *service.Service
	OldActive *service.Service
}

type ServiceGroup struct {
	mode        ServiceGroupMode
	Services    []*service.Service // sorted by priority
	activeIndex int                // currently active service in ActivePassive - mode
	OnPromotion func(*PromotionEvent)
}

func NewEmptyServiceGroup() *ServiceGroup {
	return &ServiceGroup{
		mode:        ActiveActive,
		Services:    make([]*service.Service, 0),
		activeIndex: -1,
	}
}

// returns the active service in ActivePassive mode. If the active index is not properly set, we return the first healthy service
func (sg *ServiceGroup) GetActive() (*service.Service, int) {
	if sg.mode == ActivePassive {
		if sg.activeIndex > -1 && sg.activeIndex < len(sg.Services) {
			return sg.Services[sg.activeIndex], sg.activeIndex
		}
		for idx, svc := range sg.Services { // first healthy service is active when active index is not valid
			if svc.IsHealthy() {
				sg.activeIndex = idx
				return svc, idx
			}
		}
	}

	for idx, svc := range sg.Services {
		if svc.IsHealthy() {
			return svc, idx
		}
	}
	return nil, -1 // This is pretty bad, means no service is Healthy/considered up
}

func (sg *ServiceGroup) OnServiceHealthChange(service *service.Service, healthy bool) {
	if sg.mode == ActiveActive {
		return // Does not care when it is ActiveActive
	}
	active := sg.Services[sg.activeIndex]
	if !healthy && active == service { // active has gone down!
		sg.OnPromotion(sg.promoteNextHealthy())

	} else if healthy && sg.isHigherPriorityThanActive(service, active) {
		err := sg.PromoteService(service)
		if err != nil {
			// TODO: this should in theory never hit, but how to handle?
		}

		sg.OnPromotion(&PromotionEvent{
			Service:   service.Fqdn,
			OldActive: active,
			NewActive: service,
		})
	}
}

// This does not take in to account if the registered service has the highest priority
// because a registered service is NEVER healthy at first
func (sg *ServiceGroup) RegisterService(newService *service.Service) {
	sg.Services = append(sg.Services, newService)
	slices.SortFunc(sg.Services, func(a, b *service.Service) int {
		return cmp.Compare(a.GetPriority(), b.GetPriority())
	})
	sg.SetGroupMode()
}

func (sg *ServiceGroup) PromoteService(service *service.Service) error {
	if !service.IsHealthy() {
		return ErrCannotPromoteUnHealthyService
	}
	for i, svc := range sg.Services {
		if svc.Datacenter == service.Datacenter { // Fqdn are equal anyway
			sg.activeIndex = i
			return nil
		}
	}
	return ErrServiceNotFoundInGroup
}

func (sg *ServiceGroup) promoteNextHealthy() *PromotionEvent {
	oldActive := sg.Services[sg.activeIndex]

	// Try to find next healthy service with highest priority (lowest priority number)
	bestIdx := -1
	bestPriority := int(^uint(0) >> 1) // max int

	for i, svc := range sg.Services {
		if i != sg.activeIndex && svc.IsHealthy() && svc.GetPriority() < bestPriority {
			bestIdx = i
			bestPriority = svc.GetPriority()
		}
	}

	if bestIdx != -1 {
		sg.activeIndex = bestIdx
		return &PromotionEvent{
			Service:   oldActive.Fqdn,
			NewActive: sg.Services[bestIdx],
			OldActive: oldActive,
		}
	}
	return nil
}

func (sg *ServiceGroup) isHigherPriorityThanActive(service, active *service.Service) bool {
	if !active.IsHealthy() { // if active not healthy then everyother healthy service is prioritized
		return true
	}

	if !service.IsHealthy() {
		return false
	}

	return service.GetPriority() < active.GetPriority()
}

// Will configure group mode, based on the state of group members (Services).
// If the state of the group deviates from the requirements of its mode, the mode will change
func (sg *ServiceGroup) SetGroupMode() {
	numServices := len(sg.Services)

	// If no services, default to ActiveActive
	if numServices <= 1 {
		sg.mode = ActiveActive
		sg.activeIndex = -1
		return
	}

	// Check if all services have the same priority (ActiveActive requirement)
	allSamePriority := true
	if numServices > 0 {
		firstPriority := sg.Services[0].GetPriority()
		for _, svc := range sg.Services[1:] {
			if svc.GetPriority() != firstPriority {
				allSamePriority = false
				break
			}
		}
	}

	switch sg.mode {
	case ActiveActive:
		// If services have different priorities, switch to ActivePassive
		if !allSamePriority {
			sg.mode = ActivePassive
			sg.activeIndex = 0 // Initialize to first service, which also has the highest priority!
			// does not need promotion due to highest priority service already scheduled on its intervall
		}

	case ActivePassive:
		// If all services have same priority, can switch to ActiveActive
		if allSamePriority {
			sg.mode = ActiveActive
			sg.activeIndex = -1 // Not used in ActiveActive
		} else if sg.activeIndex == -1 || sg.activeIndex >= numServices {
			// Ensure activeIndex is valid
			_, idx := sg.GetActive()
			sg.activeIndex = idx
		}

	case ActiveActivePassive:
		// TODO: implement when requirements are defined
		sg.mode = ActiveActive
		sg.activeIndex = -1

	default:
		sg.mode = ActiveActive
		sg.activeIndex = -1
	}
}
