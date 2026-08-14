package group

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

/*
type ServiceGroup struct {
	Name string
	uuid uuid.UUID

	mode ServiceGroupMode

	// sorted by priority.
	// if two services have the same priority, then the prioritizedDatacenter will decide who gets sorted into what index.
	Members []*service.Service

	// active is the service that currently holds the active role in a group.
	// In ActivePassive this is straightforward.
	// In ActiveActive it is the service that currently has the lowest roundtrip time
	active *service.Service

	//last active service in a service group
	lastActive *service.Service

	// should never receive a nil promotion event
	OnPromotion           func(*ServiceGroup)
	prioritizedDatacenter string

	hasOverride  bool
	overrideAddr ip.Address
}

func NewEmptyServiceGroup(name string) *ServiceGroup {
	// deterministic uuid generation from group name
	hash := md5.Sum([]byte(name))
	id, err := uuid.FromBytes(hash[:])
	if err != nil {
		bslog.Error("failed to generate uuid", slog.String("reason", err.Error()), slog.String("memberOf", name))
	}

	return &ServiceGroup{
		Name:         name,
		uuid:         id,
		mode:         ActiveActive,
		Members:      make([]*service.Service, 0),
		active:       nil,
		lastActive:   nil,
		hasOverride:  false,
		overrideAddr: nil,
	}
}

func (sg *ServiceGroup) Group() *model.GSLBServiceGroup {
	group := &model.GSLBServiceGroup{
		Active:      make(map[string]string),
		HasOverride: sg.hasOverride,
		Members:     make(map[string]model.GSLBService),
		UUID:        sg.uuid,
	}

	if sg.active != nil {
		group.Active[config.SplitDNS().DefaultView()] = sg.active.GetID()
	}

	if sg.hasOverride {
		group.Active[config.SplitDNS().DefaultView()] = sg.overrideAddr.String()
	}

	for _, member := range sg.Members {
		group.Members[member.GetID()] = *member.GSLBService()
	}

	return group
}

func (sg *ServiceGroup) SetOnPromotion(fn func(*ServiceGroup)) {
	sg.OnPromotion = func(g *ServiceGroup) {
		// no promotion event if override
		if sg.hasOverride {
			return
		}
		fn(sg)
	}
}

// returns active service for the group
func (sg *ServiceGroup) GetActive() *service.Service {
	if sg.active != nil {
		return sg.active
	}

	return sg.firstHealthy()
}

// returns the first healthy service of the members in the group.
// In other words, the service that SHOULD be active.
// this is true because the members are sorted on priority.
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
			sg.promoteNextHealthy()
			sg.OnPromotion(sg)
			return
		}

		if healthy && sg.triggerPromotion(changedService) {
			sg.lastActive = sg.active
			sg.active = changedService
			sg.OnPromotion(sg)
			return
		}

	case ActiveActive:
		if healthy {
			// If prioritized DC service becomes healthy, it must become active (single DNS record).
			if changedService.Datacenter == sg.prioritizedDatacenter && changedService != sg.active {
				sg.OnPromotion(sg)
				sg.lastActive = sg.active
				sg.active = changedService
				return
			}
			// If there is no active or the current active is unhealthy, promote this healthy service.
			if sg.active == nil || !sg.active.IsHealthy() {
				sg.OnPromotion(sg)
				sg.lastActive = sg.active
				sg.active = changedService
				return
			}
			return
		}

		// unhealthy
		if changedService.GetID() == sg.active.GetID() {
			next := sg.firstHealthy()
			if next != nil {
				sg.OnPromotion(sg)
				sg.lastActive = sg.active
				sg.active = next
				return
			}

			// all down
			sg.OnPromotion(sg)
			sg.lastActive = sg.active
			sg.active = nil
			return
		}
	}
}

// This does not take in to account if the registered service has the highest priority
func (sg *ServiceGroup) RegisterService(newService *service.Service) {
	if newService == nil {
		return
	}

	if sg.memberExists(newService) {
		return
	}

	sg.Members = append(sg.Members, newService)

	sg.Update()
	serviceGroupMembers.WithLabelValues(newService.MemberOf).Inc()
}

func (sg *ServiceGroup) RemoveService(id string) bool {
	members := sg.Members

	idx := slices.IndexFunc(members, func(s *service.Service) bool {
		return s.GetID() == id
	})
	if idx != -1 {
		removed := members[idx]
		sg.Members = append(members[:idx], members[idx+1:]...)
		sg.Update()
		serviceGroupMembers.WithLabelValues(sg.Name).Dec()

		events.Emit(&events.Event{
			Type:      domainEvents.EventTypeGSLBServiceMemberRemove,
			Payload:   domainEvents.GSLBServiceMemberRemoveEvent{Service: removed.MemberOf, Removed: *removed.GSLBService()},
			Timestamp: time.Now(),
			ID:        events.ID(domainEvents.EventTypeGSLBServiceMemberAdd, removed.MemberOf),
		})
	}

	return len(sg.Members) == 0
}

func (sg *ServiceGroup) promoteNextHealthy() {
	bslog.Debug("promoting next healthy service", slog.Any("oldActive", sg.active))
	//oldActive := sg.active

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
		return
	}

	// No healthy services: signal DNS delete (NewActive=nil)
	sg.active = nil
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
	numServices := len(sg.Members)
	if numServices == 0 {
		sg.mode = ActiveActive
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
		}

	case ActivePassive:
		// If all services have same priority, can switch to ActiveActive
		if allSamePriority {
			sg.mode = ActiveActive
			// if none healthy, leave active nil
		}

	/*
		case ActiveActivePassive:
			// TODO: implement when requirements are defined
			sg.mode = ActiveActive
			sg.active = sg.firstHealthy()
*/
/*
	default:
		sg.mode = ActiveActive
	}
	bslog.Debug("servicegroup mode set", slog.Any("mode", sg.mode.String()))
}

func (sg *ServiceGroup) memberExists(member *service.Service) bool {
	return slices.Contains(sg.Members, member)
}

func (sg *ServiceGroup) Update() {
	if len(sg.Members) == 0 { // dont need to do anything, group should be removed!
		return
	}

	slices.SortFunc(sg.Members, sortMembersFunc)

	sg.SetGroupMode()
	firstHealthy := sg.firstHealthy() // who should have the active role!
	if firstHealthy != sg.active {
		// trigger promotion because whoever is active should not be active anymore!
		sg.lastActive = sg.active
		sg.active = firstHealthy

		sg.OnPromotion(sg)
	}
}
*/
