package manager

import "github.com/vitistack/gslb-operator/internal/service"

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
	Services    []*service.Service
	activeIndex int // currently active service in ActivePassive - mode
	OnPromotion func(*PromotionEvent)
}

func NewEmptyServiceGroup() *ServiceGroup {
	return &ServiceGroup{
		mode:        ActiveActive,
		Services:    make([]*service.Service, 0),
		activeIndex: -1,
	}
}

func (sg *ServiceGroup) GetActive() *service.Service {
	if sg.mode == ActivePassive {
		active := sg.Services[sg.activeIndex]
		if active.IsHealthy() {
			return active
		}
	}

	for _, service := range sg.Services {
		if service.IsHealthy() {
			return service
		}
	}
	return nil // This is pretty bad, means no service is Healthy/considered up
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
		sg.OnPromotion(sg.promoteNextHealthy())
	}
}

// This does not take in to account if the registered service has the highest priority
// because a registered service is NEVER healthy at first
func (sg *ServiceGroup) RegisterService(service *service.Service) {
	sg.Services = append(sg.Services, service)
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
		if i != sg.activeIndex && svc.IsHealthy() && svc.Priority < bestPriority {
			bestIdx = i
			bestPriority = svc.Priority
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
	return service.Priority < active.Priority
}

// Will configure group mode, based on the state of group members (Services).
// If the state of the group deviates from the requirements of its mode, the mode will change
func (sg *ServiceGroup) SetGroupMode() {
	numServices := len(sg.Services)

	// If no services, default to ActiveActive
	if numServices == 0 {
		sg.mode = ActiveActive
		sg.activeIndex = -1
		return
	}

	// Check if all services have the same priority (ActiveActive requirement)
	allSamePriority := true
	if numServices > 0 {
		firstPriority := sg.Services[0].Priority
		for _, svc := range sg.Services[1:] {
			if svc.Priority != firstPriority {
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
			sg.activeIndex = 0 // Initialize to first service
			// Promote the highest priority healthy service
			sg.OnPromotion(sg.promoteNextHealthy())
		}

	case ActivePassive:
		// If all services have same priority, can switch to ActiveActive
		if allSamePriority {
			sg.mode = ActiveActive
			sg.activeIndex = -1 // Not used in ActiveActive
		} else if sg.activeIndex == -1 || sg.activeIndex >= numServices {
			// Ensure activeIndex is valid
			sg.activeIndex = 0
			sg.OnPromotion(sg.promoteNextHealthy())
		}

	case ActiveActivePassive:
		// TODO: implement when requirements are defined
		sg.mode = ActiveActive

	default:
		sg.mode = ActiveActive
		sg.activeIndex = -1
	}
}
