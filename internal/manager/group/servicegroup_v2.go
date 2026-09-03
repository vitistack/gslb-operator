package group

import (
	"cmp"
	"crypto/md5"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	domainEvents "github.com/vitistack/gslb-operator/internal/model/events"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/ip"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/iter"
)

var (
	ErrOverrideAlreadyActive = errors.New("group already has an active override for view")
)

type ServiceGroup interface {
	ID() string
	Name() string
	// register new member of group
	RegisterMember(*service.Service)

	// returns wether the group is empty or not after remove
	RemoveMember(string) bool

	// action to take when a members health changed
	OnServiceHealthChange(changedService *service.Service, healthy bool)

	// sets the action to take when a promotion in the group has happened
	SetOnPromotion(fn func(ServiceGroup, string))

	HasOverride(view string) bool
	SetOverride(view string, addr ip.Address) error
	ClearOverride(view string) *service.Service

	Seed(view, id string)
	SeededActive(view string) (string, bool)
	ClearSeed(view string)

	// returns active member for an optional view, if one exists
	// if no view is provided the default view is used
	GetActive(...string) *service.Service

	// returns the last active member for an optional view, if one exists
	// if no view is provided the default view is used
	GetLastActive(...string) *service.Service

	Members(view string) iter.Iterator[*service.Service]

	// Refresh re-sorts membership and recomputes mode/active for the given views;
	// call after an in-place config change (e.g. priority) that doesn't add or remove a member
	Refresh(...string)

	// builds and returns the service group for persistence layer
	Group() *model.GSLBServiceGroup
}

type ServiceGroupV2 struct {
	name string

	uuid uuid.UUID

	modeByView map[string]ServiceGroupMode

	members []*service.Service

	seededActive     map[string]string
	activeByView     map[string]*service.Service
	lastActiveByView map[string]*service.Service

	onPromotion func(ServiceGroup, string)

	hasOverrideByView  map[string]bool
	overrideAddrByView map[string]ip.Address
}

func NewServiceGroup(name string) *ServiceGroupV2 {
	hash := md5.Sum([]byte(name))
	id, err := uuid.FromBytes(hash[:])
	if err != nil {
		bslog.Error("failed to generate uuid", slog.String("reason", err.Error()), slog.String("memberOf", name))
	}

	return &ServiceGroupV2{
		name:               name,
		uuid:               id,
		members:            make([]*service.Service, 0),
		modeByView:         make(map[string]ServiceGroupMode),
		seededActive:       make(map[string]string),
		activeByView:       make(map[string]*service.Service),
		lastActiveByView:   make(map[string]*service.Service),
		hasOverrideByView:  make(map[string]bool),
		overrideAddrByView: make(map[string]ip.Address),
	}
}

func (sg *ServiceGroupV2) Name() string {
	return sg.name
}

func (sg *ServiceGroupV2) ID() string {
	return sg.uuid.String()
}

func (sg *ServiceGroupV2) Group() *model.GSLBServiceGroup {
	group := &model.GSLBServiceGroup{
		Active:  make(map[string]string),
		Members: make(map[string]model.GSLBService),
		Views:   make([]string, 0),
		UUID:    sg.uuid,
	}

	for view, active := range sg.activeByView {
		if active != nil {
			group.Active[view] = active.GetID()
		}
	}

	for view, addr := range sg.overrideAddrByView {
		if sg.hasOverrideByView[view] {
			group.Active[view] = addr.String()
			group.HasOverride = true
		}
	}

	for _, member := range sg.members {
		group.Members[member.GetID()] = *member.GSLBService()
		for _, view := range member.Views {
			if !slices.Contains(group.Views, view) {
				group.Views = append(group.Views, view)
			}
		}
	}

	return group
}

func (sg *ServiceGroupV2) SetOnPromotion(fn func(ServiceGroup, string)) {
	sg.onPromotion = func(group ServiceGroup, view string) {
		if sg.hasOverrideByView[view] {
			return
		}
		fn(group, view)
	}
}

func (sg *ServiceGroupV2) HasOverride(view string) bool {
	return sg.hasOverrideByView[view]
}

func (sg *ServiceGroupV2) SetOverride(view string, addr ip.Address) error {
	if sg.hasOverrideByView[view] {
		return ErrOverrideAlreadyActive
	}

	sg.hasOverrideByView[view] = true
	sg.overrideAddrByView[view] = addr
	return nil
}

func (sg *ServiceGroupV2) ClearOverride(view string) *service.Service {
	delete(sg.hasOverrideByView, view)
	delete(sg.overrideAddrByView, view)
	return sg.activeByView[view]
}

func (sg *ServiceGroupV2) OnServiceHealthChange(changedService *service.Service, healthy bool) {
	for _, view := range changedService.Views {
		sg.onServiceHealthChangeForView(view, changedService, healthy)
	}
}

func (sg *ServiceGroupV2) onServiceHealthChangeForView(view string, changedService *service.Service, healthy bool) {
	active := sg.activeByView[view]

	switch sg.modeByView[view] {
	case ActivePassive:
		if !healthy && active != nil && active.GetID() == changedService.GetID() {
			sg.promoteNextHealthy(view)
			return
		}
		if healthy && sg.triggerPromotion(view, changedService) {
			sg.promote(view, changedService)
			return
		}

	default: // ActiveActive
		if healthy && sg.triggerPromotion(view, changedService) {
			sg.promote(view, changedService)
		} else if active != nil && changedService.GetID() == active.GetID() { // current active is down
			sg.promote(view, firstHealthyOf(sg.Members(view))) // nil if all members are down
		}
	}
}

func (sg *ServiceGroupV2) Seed(view, activeID string) {
	sg.seededActive[view] = activeID
}

func (sg *ServiceGroupV2) SeededActive(view string) (string, bool) {
	id, ok := sg.seededActive[view]
	return id, ok
}

func (sg *ServiceGroupV2) ClearSeed(view string) {
	delete(sg.seededActive, view)
}

func (sg *ServiceGroupV2) GetActive(views ...string) *service.Service {
	view := config.DNS().DefaultView()
	if len(views) > 0 {
		view = views[0]
	}

	if active, ok := sg.activeByView[view]; ok && active != nil {
		return active
	}

	return firstHealthyOf(sg.Members(view))
}

func (sg *ServiceGroupV2) GetLastActive(views ...string) *service.Service {
	view := config.DNS().DefaultView()
	if len(views) > 0 {
		view = views[0]
	}

	if lastActive, ok := sg.lastActiveByView[view]; ok && lastActive != nil {
		return lastActive
	}

	return nil
}

func (sg *ServiceGroupV2) Members(view string) iter.Iterator[*service.Service] {
	return iter.FromSlice(sg.members).Filter(func(s *service.Service) bool { return slices.Contains(s.Views, view) })
}

func (sg *ServiceGroupV2) Refresh(views ...string) {
	slices.SortFunc(sg.members, sortmembersFunc)
	for _, view := range views {
		sg.updateView(view)
	}
}

func (sg *ServiceGroupV2) RegisterMember(newMember *service.Service) {
	if newMember == nil || sg.memberExists(newMember) {
		return
	}

	sg.members = append(sg.members, newMember)
	sg.Refresh(newMember.Views...)
	serviceGroupMembers.WithLabelValues(sg.name).Inc()
}

func (sg *ServiceGroupV2) RemoveMember(id string) bool {
	idx := slices.IndexFunc(sg.members, func(s *service.Service) bool {
		return s.GetID() == id
	})

	if idx == -1 {
		return len(sg.members) == 0
	}

	removed := sg.members[idx]
	sg.members = append(sg.members[:idx], sg.members[idx+1:]...)

	for _, view := range removed.Views {
		sg.updateView(view)
	}

	events.Emit(&events.Event{
		Type:      domainEvents.EventTypeGSLBServiceMemberRemove,
		Payload:   domainEvents.GSLBServiceMemberRemoveEvent{Service: sg.name, Removed: *removed.GSLBService()},
		Timestamp: time.Now(),
		ID:        events.ID(domainEvents.EventTypeGSLBServiceMemberAdd, sg.name),
	})
	serviceGroupMembers.WithLabelValues(sg.name).Dec()

	return len(sg.members) == 0
}

func (sg *ServiceGroupV2) updateView(view string) {
	if !sg.setGroupModeForView(view) {
		delete(sg.activeByView, view)
		return
	}

	firstHealthy := firstHealthyOf(sg.Members(view))
	if firstHealthy != sg.activeByView[view] {
		sg.promote(view, firstHealthy)
	}
}

// setGroupModeForView recomputes mode in a single pass; returns false if view has no members
func (sg *ServiceGroupV2) setGroupModeForView(view string) bool {
	count, firstPriority := 0, 0
	allSamePriority := true

	for m := range sg.Members(view) {
		if count == 0 {
			firstPriority = m.GetPriority()
		} else if m.GetPriority() != firstPriority {
			allSamePriority = false
		}
		count++
	}

	if count == 0 {
		delete(sg.modeByView, view)
		return false
	}
	if count == 1 {
		sg.modeByView[view] = ActiveActive
		return true
	}

	switch sg.modeByView[view] {
	case ActivePassive:
		if allSamePriority {
			sg.modeByView[view] = ActiveActive
		}
	default:
		if allSamePriority {
			sg.modeByView[view] = ActiveActive
		} else {
			sg.modeByView[view] = ActivePassive
		}
	}
	return true
}

func (sg *ServiceGroupV2) promote(view string, newActive *service.Service) {
	sg.lastActiveByView[view] = sg.activeByView[view]
	sg.activeByView[view] = newActive
	sg.onPromotion(sg, view)
}

func (sg *ServiceGroupV2) promoteNextHealthy(view string) {
	var best *service.Service
	bestPriority := int(^uint(0) >> 1)

	for m := range sg.Members(view) {
		if m.IsHealthy() && m.GetPriority() < bestPriority {
			best, bestPriority = m, m.GetPriority()
		}
	}

	sg.promote(view, best)
}

// returns wether the current service healthchange should trigger a promotion
func (sg *ServiceGroupV2) triggerPromotion(view string, svc *service.Service) bool {
	if !svc.IsHealthy() {
		return false
	}

	active := sg.activeByView[view]
	if active == nil || !active.IsHealthy() {
		return true
	}

	if svc.GetPriority() < active.GetPriority() {
		return true
	}

	return svc.GetAverageRoundtrip() < active.GetAverageRoundtrip()
}

func (sg *ServiceGroupV2) memberExists(svc *service.Service) bool {
	return slices.Contains(sg.members, svc)
}

// func passed into slices.SortFunc for sorting the groups members
func sortmembersFunc(a, b *service.Service) int {
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

func firstHealthyOf(members iter.Iterator[*service.Service]) *service.Service {
	for member := range members {
		if member.IsHealthy() {
			return member
		}
	}
	return nil
}
