package manager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/dns/update"
	"github.com/vitistack/gslb-operator/internal/manager/group"
	"github.com/vitistack/gslb-operator/internal/manager/healthcheck"
	"github.com/vitistack/gslb-operator/internal/manager/scheduler"
	"github.com/vitistack/gslb-operator/internal/model"
	domainEvents "github.com/vitistack/gslb-operator/internal/model/events"
	"github.com/vitistack/gslb-operator/internal/repositories/servicegroup"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence/store/memory"
	"github.com/vitistack/gslb-operator/pkg/pool"
)

// Responsible for managing services, on scheduling services for health checks
type ServicesManager struct {
	// servicesHealthCheck maps check intervals to services that should be checked at that interval.
	scheduledServices  ScheduledServices                           // services that are scheduled on an interval
	schedulers         map[timesutil.Duration]*scheduler.Scheduler // schedulers for health-checks
	serviceGroups      group.ServiceGroups
	healthChangeEvents chan *service.HealthChangeEvent

	svcGroupRepo *servicegroup.ServiceGroupRepo

	mutex      sync.RWMutex
	groupLocks map[string]sync.Mutex
	stop       sync.Once
	wg         *sync.WaitGroup // schedulers use this when scheduling services asynchronously

	DNSCreate func(...update.Record) error
	DNSDelete func(string, ...string) error

	pool   *pool.WorkerPool
	dryrun bool
}

func NewManager(opts ...serviceManagerOption) *ServicesManager {
	cfg := managerConfig{
		MinRunningWorkers:     1,
		NonBlockingBufferSize: 1,
		DryRun:                false,
		repo:                  servicegroup.NewServiceGroupRepo(memory.NewStore[model.GSLBServiceGroup]()),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.DryRun {
		bslog.Warn("dry-run enabled")
	}

	pool := pool.NewWorkerPool(cfg.MinRunningWorkers, cfg.NonBlockingBufferSize)
	pool.OnScaleUp = func() {
		bslog.Debug("worker-pool on scale up", slog.Int("numWorkers", int(pool.NumWorkers())))
		workerPoolSize.Inc()
	}
	pool.OnScaleDown = func() {
		bslog.Debug("worker-pool on scale down", slog.Int("numWorkers", int(pool.NumWorkers())))
		workerPoolSize.Dec()
	}

	mgr := &ServicesManager{
		scheduledServices:  make(ScheduledServices),
		schedulers:         make(map[timesutil.Duration]*scheduler.Scheduler),
		serviceGroups:      make(group.ServiceGroups),
		healthChangeEvents: make(chan *service.HealthChangeEvent, cfg.MinRunningWorkers),
		svcGroupRepo:       cfg.repo,
		mutex:              sync.RWMutex{},
		groupLocks:         make(map[string]sync.Mutex),
		stop:               sync.Once{},
		wg:                 &sync.WaitGroup{},
		pool:               pool,
		dryrun:             cfg.DryRun,
	}

	return mgr
}

// Start begins scheduling health checks for all services according to their configured intervals.
// It ensures that the scheduling logic is only executed once, even if called multiple times.
func (sm *ServicesManager) Start(ctx context.Context) {
	sm.pool.Start()
	sm.wg.Go(func() {
		sm.handleServiceHealthChange(ctx)
	})
}

func (sm *ServicesManager) Stop() {
	sm.stop.Do(func() {
		for _, scheduler := range sm.schedulers {
			scheduler.Stop()
		}
		bslog.Debug("waiting for schedulers to stop")
		sm.wg.Wait()

		bslog.Debug("schedulers stopped - closing pool")
		sm.pool.Stop()
		err := sm.OnShutdown()
		if err != nil {
			bslog.Error("error while performing shutdown tasks", slog.String("error", err.Error()))
		}
		bslog.Debug("service manager closed")
	})
}

func (sm *ServicesManager) OnShutdown() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	bslog.Debug("executing manager.OnShutdown()")

	updateErrors := make([]error, 0, len(sm.serviceGroups))
	for _, group := range sm.serviceGroups {
		updateErr := sm.svcGroupRepo.Update(group.Name(), group.Group())
		if updateErr != nil {
			updateErrors = append(updateErrors, fmt.Errorf("failed to update service group: %s: %w", group.Name(), updateErr))
		}
	}

	if len(updateErrors) > 0 {
		return errors.Join(updateErrors...)
	}

	return nil
}

func (sm *ServicesManager) RegisterService(serviceCfg model.GSLBConfig) (*service.Service, error) {
	sm.mutex.RLock()
	_, _, oldSvc := sm.scheduledServices.Search(serviceCfg.ServiceID)
	if oldSvc != nil { // update service if already exists
		sm.mutex.RUnlock()
		sm.updateService(oldSvc, serviceCfg)
		return oldSvc, nil
	}
	sm.mutex.RUnlock()

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	newService, err := service.NewServiceFromGSLBConfig(serviceCfg, sm.BuildServiceOptions(serviceCfg)...)
	if err != nil {
		return nil, fmt.Errorf("unable to register service: %s", err.Error())
	}

	svcGroup, err := sm.svcGroupRepo.Read(serviceCfg.MemberOf)
	if err != nil {
		return nil, fmt.Errorf("failed to register config: %w", err)
	}
	_, exists := svcGroup.Members[serviceCfg.ServiceID]
	if svcGroup.Members == nil {
		svcGroup.Members = make(map[string]model.GSLBService)
	}
	svcGroup.Members[serviceCfg.ServiceID] = *newService.GSLBService()

	// create new service group if needed, and register service in group
	sm.newServiceGroup(newService.MemberOf).RegisterMember(newService)

	// create/update service group
	err = sm.svcGroupRepo.Create(serviceCfg.MemberOf, &svcGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to create new service: %w", err)
	}

	// set healthchange callback action
	newService.SetHealthChangeCallback(func(event *service.HealthChangeEvent) {
		// push event to handler
		sm.healthChangeEvents <- event
	})

	newService.SetFailureCountCallback(func(svc *model.GSLBService) {
		sm.wg.Go(func() {
			if err := sm.svcGroupRepo.UpdateMember(svc.MemberOf, *svc); err != nil {
				bslog.Error("failed to update service failurecount",
					slog.String("reason", err.Error()),
					slog.Any("service", svc),
				)
			}
		})
	})

	// create new scheduler if needed, and schedule service for health-checks
	sm.newScheduler(newService.ScheduledInterval).ScheduleService(newService)

	// register the service in the datastructure
	sm.scheduledServices.Add(newService)

	bslog.Debug("registered service", slog.Any("service", newService))
	// only emit events if the config was never registered in the first place
	if !exists {
		events.Emit(
			&events.Event{ // publish service registration event
				Type: domainEvents.EventTypeGSLBConfigCreate,
				Payload: domainEvents.GSLBConfigCreateEvent{
					Config: serviceCfg,
				},
				Timestamp: time.Now(),
				ID:        events.ID(domainEvents.EventTypeGSLBConfigCreate, serviceCfg.ServiceID),
			},
			&events.Event{
				Type: domainEvents.EventTypeGSLBServiceMemberAdd,
				Payload: domainEvents.GSLBServiceMemberAddEvent{
					Service:   newService.MemberOf,
					NewMember: *newService.GSLBService(),
				},
				Timestamp: time.Now(),
				ID:        events.ID(domainEvents.EventTypeGSLBServiceMemberAdd, newService.MemberOf),
			},
		)
	}

	return newService, nil
}

// removes the service from its healthcheck queue
func (sm *ServicesManager) RemoveService(id string) error {
	sm.mutex.RLock()
	_, interval, svc := sm.scheduledServices.Search(id)
	sm.mutex.RUnlock()
	if svc == nil { // cannot remove something that does not exists
		return ErrServiceNotFound
	}

	sm.scheduledServices.Delete(id)
	sm.schedulers[interval].RemoveService(svc) // remove the service from its scheduler

	sm.mutex.RLock()
	group := sm.serviceGroups[svc.MemberOf]
	sm.mutex.RUnlock()

	empty := group.RemoveMember(svc.GetID()) // registered in group
	if empty {
		sm.deleteGroup(svc.MemberOf)

		bslog.Debug("removed service", slog.Any("service", svc))
		events.Emit(&events.Event{ // publish delete event for service
			Type: domainEvents.EventTypeGSLBConfigDelete,
			Payload: domainEvents.GSLBConfigDeleteEvent{
				LastConfig: svc.GSLBConfig(),
			},
			Timestamp: time.Now(),
			ID:        events.ID(domainEvents.EventTypeGSLBConfigDelete, svc.GetID()),
		})
		return nil
	}

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	err := sm.svcGroupRepo.Update(svc.MemberOf, group.Group())
	if err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	bslog.Debug("removed service", slog.Any("service", svc))
	events.Emit(&events.Event{ // publish delete event for service
		Type: domainEvents.EventTypeGSLBConfigDelete,
		Payload: domainEvents.GSLBConfigDeleteEvent{
			LastConfig: svc.GSLBConfig(),
		},
		Timestamp: time.Now(),
		ID:        events.ID(domainEvents.EventTypeGSLBConfigDelete, svc.GetID()),
	})

	return nil
}

// updates an existing service with new configuration
func (sm *ServicesManager) updateService(old *service.Service, cfg model.GSLBConfig) {
	new, err := service.NewServiceFromGSLBConfig(cfg, sm.BuildServiceOptions(cfg)...)
	if err != nil {
		bslog.Error("failed to update service with new config", slog.String("reason", err.Error()))
		return
	}

	sm.mutex.Lock()
	if !old.ConfigChanged(cfg) { // nothing to do
		bslog.Debug("skipping update due to unchanged config", slog.Any("service", old))
		sm.mutex.Unlock()
		return
	}

	oldDefaultInterval, newDefaultInterval := old.GetDefaultInterval(), new.GetDefaultInterval()
	oldMemberOf, newMemberOf := old.MemberOf, new.MemberOf

	lastConfig := old.GSLBConfig()
	views := old.Views
	old.Assign(new) // assigning changed config variables to the registered service
	sm.mutex.Unlock()

	if oldMemberOf != newMemberOf {
		sm.memberOfChanged(oldMemberOf, newMemberOf, old)
	} else {
		sm.mutex.RLock()
		oldGroup, ok := sm.serviceGroups[oldMemberOf]
		sm.mutex.RUnlock()
		if ok {
			views = append(views, old.Views...)
			slices.Sort(views)
			// update all views that may have had an effect
			oldGroup.Refresh(slices.Compact(views)...)
		} else { // this will probably never run, but you never know in concurrency!
			sm.deleteGroup(oldMemberOf)
		}
	}

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	oldGroup, ok := sm.serviceGroups[oldMemberOf]
	if ok {
		err = sm.svcGroupRepo.Update(oldMemberOf, oldGroup.Group())
		if err != nil {
			bslog.Error(
				"failed to update servicegroup config persistently",
				slog.String("reason", err.Error()),
				slog.Any("group", oldGroup),
			)
		}
	}

	// important that this checked AFTER the service groups have ran their update
	// this is because the group may trigger a promotion event that needs to be handled first
	// if the promotion event does not happen, we just simply move it to a new interval
	if oldDefaultInterval != newDefaultInterval && oldDefaultInterval == old.ScheduledInterval {
		// we need to move the service to a new interval
		// otherwise the service will get rescheduled back to its default interval on its own, when it is needed
		sm.moveServiceToInterval(old, newDefaultInterval)
	}

	bslog.Debug("updated service", slog.Any("service", old))
	events.Emit(&events.Event{ // publish configuration change event
		Type: domainEvents.EventTypeGSLBConfigUpdate,
		Payload: domainEvents.GSLBConfigUpdateEvent{
			LastConfig:    lastConfig,
			CurrentConfig: cfg,
		},
		Timestamp: time.Now(),
		ID:        events.ID(domainEvents.EventTypeGSLBConfigUpdate, new.GetID()),
	})
}

func (sm *ServicesManager) memberOfChanged(oldMemberOf, newMemberOf string, svc *service.Service) {
	sm.mutex.Lock()
	oldGroup, oldOk := sm.serviceGroups[oldMemberOf]
	newGroup := sm.newServiceGroup(newMemberOf)
	sm.mutex.Unlock()

	// Register in new group and persist its state
	newGroup.RegisterMember(svc)
	if err := sm.svcGroupRepo.Create(newMemberOf, newGroup.Group()); err != nil {
		bslog.Error(
			"failed to persist new service group membership",
			slog.String("reason", err.Error()),
			slog.String("newMemberOf", newMemberOf),
			slog.Any("service", svc),
		)
		return
	}

	events.Emit(&events.Event{
		Type: domainEvents.EventTypeGSLBServiceMemberAdd,
		Payload: domainEvents.GSLBServiceMemberAddEvent{
			Service:   svc.MemberOf,
			NewMember: *svc.GSLBService(),
		},
		Timestamp: time.Now(),
		ID:        events.ID(domainEvents.EventTypeGSLBServiceMemberAdd, svc.MemberOf),
	})

	// Clean up old group
	if !oldOk {
		bslog.Debug("updated service group membership",
			slog.String("oldGroup", oldMemberOf),
			slog.String("newGroup", newMemberOf),
		)
		return
	}

	empty := oldGroup.RemoveMember(svc.GetID())
	if empty {
		sm.deleteGroup(oldMemberOf)
	} else {
		if err := sm.svcGroupRepo.Update(oldMemberOf, oldGroup.Group()); err != nil {
			bslog.Error(
				"failed to update old service group after member removal",
				slog.String("reason", err.Error()),
				slog.String("oldMemberOf", oldMemberOf),
				slog.Any("service", svc),
			)
		}
	}

	bslog.Debug("updated service group membership",
		slog.String("oldGroup", oldMemberOf),
		slog.String("newGroup", newMemberOf),
	)
}

func (sm *ServicesManager) handleServiceHealthChange(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			bslog.Debug("stopped service healthchange management")
			return

		case healthchange, ok := <-sm.healthChangeEvents:
			if !ok {
				bslog.Debug("healthchange events closed")
				return
			}
			sm.ServiceHealthChangeCallback(healthchange)
		}
	}
}

// reconciles the current state of a group to reflect internal state of the manager
// and external state
func (sm *ServicesManager) reconcile(group group.ServiceGroup, view string) {
	err := sm.svcGroupRepo.Update(group.Name(), group.Group())
	if err != nil {
		bslog.Error("failed to reconcile service-group",
			slog.String("reason", err.Error()),
			slog.Any("group", group),
		)
		return
	}

	// remove all DNS reference for the group
	/*
	* conscious decision to leave out view here:
	* because if the record is not created later then it should have not existed in the first place
	* e.g. we clean it up here. (late non the less)
	*
	* or if all the sites are down, then it should be down from ALL views
	 */
	err = sm.DNSDelete(group.ID())
	if err != nil {
		bslog.Error("failed to reconcile gslb service-group",
			slog.String("reason", fmt.Errorf("failed to delete DNS records: %w", err).Error()),
			slog.Any("group", group))
		return
	}

	active := group.GetActive(view)
	if active == nil {
		// all services for the group is unhealthy
		bslog.Warn("gslb service group is down",
			slog.String("service", group.Name()),
			slog.String("view", view),
			slog.String("status", "down"),
			slog.String("reason", "all members are considered down"),
		)

		events.Emit(&events.Event{
			Type: domainEvents.EventTypeGSLBServiceDown,
			Payload: domainEvents.GSLBServiceDownEvent{
				MemberOf: group.Name(),
			},
			Timestamp: time.Now(),
			ID:        events.ID(domainEvents.EventTypeGSLBServiceDown, group.Name()),
		})
		return
	}

	if err := sm.DNSCreate(
		update.Record{
			Name:    active.MemberOf,
			Address: active.GetAddress(),
			Views:   active.Views,
			UUID:    string(group.ID()),
		}); err != nil {
		bslog.Error("failed to reconcile gslb service-group",
			slog.String("reason", fmt.Errorf("failed to create DNS record: %w", err).Error()),
			slog.Any("activeService", active),
		)
		return
	}

	sm.reconcileHealthCheckIntervals(group, view)
}

func (sm *ServicesManager) reconcileHealthCheckIntervals(group group.ServiceGroup, view string) {
	active := group.GetActive(view)
	baseInterval := active.GetBaseInterval()

	if active.ScheduledInterval > baseInterval {
		bslog.Info("update gslb-service member healthcheck interval",
			slog.Any("member", active),
			slog.Group("intervalChange",
				slog.String("from", active.ScheduledInterval.String()),
				slog.String("to", baseInterval.String()),
			))
		sm.moveServiceToInterval(active, baseInterval)
	}

	for member := range group.Members(view) {
		if member != active && member.ScheduledInterval == baseInterval {
			demotedInterval := member.GetDefaultInterval()

			if demotedInterval == baseInterval {
				// the service with highest priority is not active
				// so by setting its scheduled interval
				// to the currently active service's default interval
				// we achieve an interval "swap"
				demotedInterval = active.GetDefaultInterval()
			}

			bslog.Info("update service healthcheck interval",
				slog.Any("member", member),
				slog.Group("intervalChange",
					slog.String("from", member.ScheduledInterval.String()),
					slog.String("to", demotedInterval.String()),
				),
			)
			sm.moveServiceToInterval(member, demotedInterval)
		}
	}

	if group.GetLastActive(view) == nil {
		events.Emit(&events.Event{
			Type:      domainEvents.EventTypeGSLBServiceUp,
			Payload:   domainEvents.GSLBServiceUpEvent{NewActive: *active.GSLBService()},
			Timestamp: time.Now(),
			ID:        events.ID(domainEvents.EventTypeGSLBServiceUp, group.Name()),
		})
		return
	}
}

func (sm *ServicesManager) newServiceGroup(memberOf string) group.ServiceGroup {
	serviceGroup, ok := sm.serviceGroups[memberOf]
	if ok {
		return serviceGroup
	}
	newGroup := group.NewServiceGroup(memberOf)
	newGroup.SetOnPromotion(
		func(group group.ServiceGroup, view string) {
			sm.wg.Go(func() {
				sm.reconcile(group, view)
			})
		},
	)

	sm.serviceGroups[memberOf] = newGroup

	serviceGroups.Inc()
	return newGroup
}

// only called when we know it is safe to delete a group
func (sm *ServicesManager) deleteGroup(memberOf string) {
	sm.mutex.Lock()
	delete(sm.serviceGroups, memberOf)
	sm.mutex.Unlock()

	err := sm.svcGroupRepo.Delete(memberOf)
	if err != nil {
		bslog.Error("failed to delete service group", slog.String("reason", err.Error()))
	}

	serviceGroups.Dec()
}

// creates a new scheduler, and starts its loop
func (sm *ServicesManager) newScheduler(interval timesutil.Duration) *scheduler.Scheduler {
	if scheduler, ok := sm.schedulers[interval]; ok { // scheduler already exists
		return scheduler
	}

	scheduler := scheduler.NewScheduler(time.Duration(interval), sm.wg)
	sm.schedulers[interval] = scheduler

	scheduler.OnTick = func(svc *service.Service) {
		err := sm.pool.Put(healthcheck.NewJob(svc))
		if errors.Is(err, pool.ErrPutOnClosedPool) {
			bslog.Error("failed to schedule health check", slog.String("reason", err.Error()))
		}
	}

	bslog.Debug("new scheduler", slog.String("interval", interval.String()))
	return scheduler
}

func (sm *ServicesManager) cleanupInterval(interval timesutil.Duration) {
	if scheduler, ok := sm.schedulers[interval]; ok {
		scheduler.Stop()
		delete(sm.schedulers, interval)
	}
	bslog.Debug("deleted scheduler", slog.String("interval", interval.String()))
}

func (sm *ServicesManager) moveServiceToInterval(svc *service.Service, newInterval timesutil.Duration) {
	oldInterval := svc.ScheduledInterval
	if oldInterval == newInterval {
		return // already scheduled on this interval
	}
	sm.scheduledServices.MoveToInterval(svc, newInterval)

	oldScheduler, newScheduler := sm.schedulers[oldInterval], sm.schedulers[newInterval]
	last := oldScheduler.RemoveService(svc)
	if last {
		sm.cleanupInterval(oldInterval)
	}

	if newScheduler == nil {
		newScheduler = sm.newScheduler(newInterval)
	}

	newScheduler.ScheduleService(svc)
	bslog.Debug("sucessfully moved service to new interval",
		slog.String("oldInterval", oldInterval.String()),
		slog.String("newInterval", newInterval.String()),
		slog.Any("service", svc))
}

func (sm *ServicesManager) GetActiveForMemberOf(memberOf string) *service.Service {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	if group, ok := sm.serviceGroups[memberOf]; ok {
		return group.GetActive()
	}
	return nil
}

func (sm *ServicesManager) BuildServiceOptions(config model.GSLBConfig) []service.ServiceOption {
	opts := make([]service.ServiceOption, 0, 5)
	opts = append(opts, service.WithDryRunChecks(sm.dryrun))

	gslbServiceGroup, err := sm.svcGroupRepo.Read(config.MemberOf)
	if err != nil {
		//if errors.Is(err, svcRepo.ErrServiceInGroupNotFound) {
		//	bslog.Debug("could not find member in group",
		//		slog.String("group", config.MemberOf),
		//		slog.String("member", config.ServiceID),
		//	)
		//}
		// max out the failure count
		// means a long time before service will be considered healthy
		opts = append(opts, service.WithFailureCount(config.FailureThreshold))
		bslog.Error("could not fetch service group from storage",
			slog.String("reason", err.Error()),
			slog.String("action", "potential failurecount reset"),
			slog.String("service", config.MemberOf),
			slog.Int("failureCount", config.FailureThreshold),
		)
		return opts
	}

	member, ok := gslbServiceGroup.Members[config.ServiceID]
	if ok {
		opts = append(opts, service.WithFailureCount(member.FailureCount))

		if member.IsHealthy {
			opts = append(opts, service.WithHealthy())
		}
		return opts
	}

	// member not found in group
	// max out failure count
	opts = append(opts, service.WithFailureCount(config.FailureThreshold))

	return opts
}

func (sm *ServicesManager) ServiceHealthChangeCallback(event *service.HealthChangeEvent) {
	bslog.Debug("received health-change", slog.Any("service", event.Svc), slog.Bool("healthy", event.Healthy))

	sm.mutex.Lock()
	group := sm.serviceGroups[event.Svc.MemberOf]
	sm.mutex.Unlock()

	err := sm.svcGroupRepo.Update(group.Name(), group.Group())
	if err != nil {
		bslog.Error(
			"failed to update service health on health-change",
			slog.String("reason", err.Error()),
			slog.Any("service", event.Svc),
		)
	}

	events.Emit(&events.Event{
		Type: domainEvents.EventTypeGSLBServiceMemberHealthChange,
		Payload: domainEvents.GSLBServiceMemberHealthChangeEvent{
			Member: *event.Svc.GSLBService(),
		},
		Timestamp: time.Now(),
		ID:        events.ID(domainEvents.EventTypeGSLBServiceMemberHealthChange, event.Svc.MemberOf),
	})

	group.OnServiceHealthChange(event.Svc, event.Healthy)
}

func (sm *ServicesManager) CreateOverride(override spoofs.Override) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	group, ok := sm.serviceGroups[override.MemberOf]

	if !ok {
		return fmt.Errorf("%w: %s", ErrServiceGroupNotFound, override.MemberOf)
	}

	view := override.View
	if view == "" {
		view = config.DNS().DefaultView()
	}

	if override.Address == nil {
		return fmt.Errorf("override address is required")
	}

	if group.HasOverride(view) {
		return fmt.Errorf("group %s already has an active override", override.MemberOf)
	}

	err := group.SetOverride(override.View, override.Address)
	if err != nil {
		return fmt.Errorf("%s: failed to set override: %w", group.Name(), err)
	}

	err = sm.svcGroupRepo.Update(override.MemberOf, group.Group())
	if err != nil {
		group.ClearOverride(view)
		return fmt.Errorf("failed to update service group: %w", err)
	}

	if err := sm.DNSCreate(
		update.Record{
			Name:    override.MemberOf,
			Address: override.Address,
			Views:   []string{view},
			UUID:    group.ID(),
		},
	); err != nil {
		return fmt.Errorf("failed to create DNS spoof: %w", err)
	}

	return nil
}

func (sm *ServicesManager) RemoveOverride(memberOf string, views ...string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	group, ok := sm.serviceGroups[memberOf]

	if !ok {
		return fmt.Errorf("%w: %s", ErrServiceGroupNotFound, memberOf)
	}

	overridenViews := make(map[string]struct{})
	for _, view := range views {
		if group.HasOverride(view) {
			overridenViews[view] = struct{}{}
		}
	}

	if config.DNS().Enable() {
		for _, view := range config.DNS().DNSViews() {
			if group.HasOverride(view) {
				overridenViews[view] = struct{}{}
			}
		}
	}

	// ensure idempotency
	if !(len(overridenViews) > 0) {
		return nil
	}

	deleteViews := make([]string, 0, len(overridenViews))
	for view := range overridenViews {
		group.ClearOverride(view)
		deleteViews = append(deleteViews, view)
	}

	err := sm.svcGroupRepo.Update(memberOf, group.Group())
	if err != nil {
		return fmt.Errorf("failed to update service group: %w", err)
	}

	// delete override spoof
	if err := sm.DNSDelete(group.ID(), deleteViews...); err != nil {
		return fmt.Errorf("failed to delete override: %w", err)
	}

	for _, view := range deleteViews {
		active := group.GetActive(view)
		if active != nil && active.IsHealthy() {
			if err := sm.DNSCreate(update.Record{
				Name:    active.MemberOf,
				Address: active.GetAddress(),
				UUID:    group.ID(),
			}); err != nil {
				return fmt.Errorf("failed to create DNS spoof: %w", err)
			}
		}
	}

	return nil
}
