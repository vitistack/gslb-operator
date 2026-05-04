package manager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/vitistack/gslb-operator/internal/manager/healthcheck"
	"github.com/vitistack/gslb-operator/internal/manager/scheduler"
	"github.com/vitistack/gslb-operator/internal/model"
	domainEvents "github.com/vitistack/gslb-operator/internal/model/events"
	svcRepo "github.com/vitistack/gslb-operator/internal/repositories/service"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/persistence/store/memory"
	"github.com/vitistack/gslb-operator/pkg/pool"
)

// Responsible for managing services, on scheduling services for health checks
type ServicesManager struct {
	// servicesHealthCheck maps check intervals to services that should be checked at that interval.
	scheduledServices  ScheduledServices                           // services that are scheduled on an interval
	schedulers         map[timesutil.Duration]*scheduler.Scheduler // schedulers for health-checks
	serviceGroups      map[string]*ServiceGroup
	healthChangeEvents chan *service.HealthChangeEvent
	svcRepo            *svcRepo.ServiceRepo
	mutex              sync.RWMutex
	stop               sync.Once
	pool               *pool.WorkerPool
	wg                 *sync.WaitGroup // schedulers use this when scheduling services asynchronously
	DNSUpdate          func(*service.Service, bool)
	dryrun             bool
}

func NewManager(opts ...serviceManagerOption) *ServicesManager {
	cfg := managerConfig{
		MinRunningWorkers:     100,
		NonBlockingBufferSize: 100,
		DryRun:                false,
		repo:                  svcRepo.NewServiceRepo(memory.NewStore[model.GSLBServiceGroup]()),
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
		serviceGroups:      make(map[string]*ServiceGroup),
		healthChangeEvents: make(chan *service.HealthChangeEvent, cfg.MinRunningWorkers),
		svcRepo:            cfg.repo,
		mutex:              sync.RWMutex{},
		pool:               pool,
		stop:               sync.Once{},
		wg:                 &sync.WaitGroup{},
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

	for memberOf, group := range sm.serviceGroups {
		active := group.GetActive()

		for _, svc := range group.Members {
			gslbService := svc.GSLBService()

			gslbService.IsActive = (active != nil && active.GetID() == svc.GetID())
			override, err := sm.svcRepo.HasOverride(memberOf)
			if err != nil {
				return fmt.Errorf("unable to check whether service group has active override: member-of: %s: %w", memberOf, err)
			}
			gslbService.HasOverride = override

			err = sm.svcRepo.Update(gslbService)
			if err != nil {
				return fmt.Errorf("failed to persist service state: service: %v: %w", svc, err)
			}
		}
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

	members, err := sm.svcRepo.Read(serviceCfg.MemberOf)
	if err != nil {
		return nil, fmt.Errorf("failed to register config: %w", err)
	}
	exists := slices.ContainsFunc(members, func(m model.GSLBService) bool { return m.ID == serviceCfg.ServiceID })

	// will update entry with new values if changed
	// therefore no need to check if it exists at this point
	err = sm.svcRepo.Create(newService.GSLBService())
	if err != nil {
		return nil, fmt.Errorf("failed to create new service: %w", err)
	}

	// set healthchange callback action
	newService.SetHealthChangeCallback(func(event *service.HealthChangeEvent) {
		// push event to handler
		sm.healthChangeEvents <- event
	})

	// create new scheduler if needed, and schedule service for health-checks
	sm.newScheduler(newService.ScheduledInterval).ScheduleService(newService)

	// register the service in the datastructure
	sm.scheduledServices.Add(newService)

	// create new service group if needed, and register service in group
	sm.newServiceGroup(newService.MemberOf).RegisterService(newService)

	bslog.Debug("registered service", slog.Any("service", newService))
	// only emit events if the config was never registered in the first place
	if !exists {
		events.Emit(&events.Event{ // publish service registration event
			Type: domainEvents.EventTypeGSLBConfigCreate,
			Payload: domainEvents.GSLBConfigCreateEvent{
				Config: serviceCfg,
			},
			Timestamp: time.Now(),
			ID:        events.ID(domainEvents.EventTypeGSLBConfigCreate, serviceCfg.ServiceID),
		})

		events.Emit(&events.Event{
			Type: domainEvents.EventTypeGSLBServiceMemberAdd,
			Payload: domainEvents.GSLBServiceMemberAddEvent{
				Service:   newService.MemberOf,
				NewMember: *newService.GSLBService(),
			},
			Timestamp: time.Now(),
			ID:        events.ID(domainEvents.EventTypeGSLBServiceMemberAdd, newService.MemberOf),
		})
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

	sm.schedulers[interval].RemoveService(svc) // remove the service from its scheduler

	sm.mutex.RLock()
	group := sm.serviceGroups[svc.MemberOf]
	sm.mutex.RUnlock()

	empty := group.RemoveService(svc.GetID()) // registered in group
	if empty {
		sm.deleteGroup(svc.MemberOf)
	}

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.scheduledServices.Delete(id)
	err := sm.svcRepo.Delete(svc.MemberOf, svc.GetID())
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
	if !old.ConfigChanged(new) { // nothing to do
		bslog.Debug("skipping update due to unchanged config", slog.Any("service", old))
		sm.mutex.Unlock()
		return
	}

	oldDefaultInterval, newDefaultInterval := old.GetDefaultInterval(), new.GetDefaultInterval()
	oldMemberOf, newMemberOf := old.MemberOf, new.MemberOf

	old.Assign(new) // assigning changed config variables to the registered service
	sm.mutex.Unlock()

	if oldMemberOf != newMemberOf {
		sm.memberOfChanged(oldMemberOf, newMemberOf, old)
	} else {
		sm.mutex.RLock()
		oldGroup, ok := sm.serviceGroups[oldMemberOf]
		sm.mutex.RUnlock()
		if ok {
			oldGroup.Update() // notify potential changes to group
		} else { // this will probably never run, but you never know in concurrency!
			sm.deleteGroup(oldMemberOf)
		}
	}

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	err = sm.svcRepo.Update(old.GSLBService())
	if err != nil {
		bslog.Error(
			"failed to update service config persistently",
			slog.String("reason", err.Error()),
			slog.Any("service", old),
		)
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
			LastConfig:    old.GSLBConfig(),
			CurrentConfig: new.GSLBConfig(),
		},
		Timestamp: time.Now(),
		ID:        events.ID(domainEvents.EventTypeGSLBConfigUpdate, new.GetID()),
	})
}

func (sm *ServicesManager) memberOfChanged(oldMemberOf, newMemberOf string, svc *service.Service) {
	sm.mutex.Lock()

	err := sm.svcRepo.Delete(oldMemberOf, svc.GetID())
	if err != nil {
		bslog.Error(
			"failed to remove service from old service group",
			slog.String("reason", err.Error()),
			slog.String("oldMemberOf", oldMemberOf),
			slog.Any("service", svc),
		)
		return
	}

	err = sm.svcRepo.Create(svc.GSLBService())
	if err != nil {
		bslog.Error(
			"failed to add service to new group",
			slog.String("reason", err.Error()),
			slog.String("newMemberOf", newMemberOf),
			slog.Any("service", svc),
		)
		return
	}

	// create new service group if needed
	newGroup := sm.newServiceGroup(newMemberOf)

	oldGroup, oldOk := sm.serviceGroups[oldMemberOf]
	sm.mutex.Unlock()

	newGroup.RegisterService(svc)
	events.Emit(&events.Event{
		Type: domainEvents.EventTypeGSLBServiceMemberAdd,
		Payload: domainEvents.GSLBServiceMemberAddEvent{
			Service:   svc.MemberOf,
			NewMember: *svc.GSLBService(),
		},
		Timestamp: time.Now(),
		ID:        events.ID(domainEvents.EventTypeGSLBServiceMemberAdd, svc.MemberOf),
	})

	var empty bool
	if oldOk {
		empty = oldGroup.RemoveService(svc.GetID())

	}

	if empty { // delete empty service group
		sm.deleteGroup(oldMemberOf)
	}

	bslog.Debug(
		"updated service group membership",
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

// re-schedules the relevant services in the PromotionEvent
func (sm *ServicesManager) handlePromotion(event *PromotionEvent) {
	var newID, oldID string
	if event.NewActive != nil {
		newID = event.NewActive.GetID()
	}

	if event.OldActive != nil {
		oldID = event.OldActive.GetID()
	}

	// No-op: nothing to change
	if newID == oldID {
		bslog.Debug("skipping promotion event", slog.String("reason", "unchanged active member"))
		return
	}

	var baseInterval timesutil.Duration

	msg := "received promotion event for service: " + event.Service + ": "
	// set baseInterval
	if event.OldActive != nil {
		msg += " OldActive: " + event.OldActive.Datacenter + " "
		baseInterval = event.OldActive.GetBaseInterval()
	}
	if event.NewActive != nil {
		msg += "NewActive: " + event.NewActive.Datacenter
		if baseInterval == 0 {
			baseInterval = event.NewActive.GetBaseInterval()
		}
	}
	bslog.Debug(msg)

	// new active member in the service group
	if event.OldActive != nil && event.NewActive != nil {
		sm.handleNewActiveMember(event, baseInterval)
		return
	}

	if event.NewActive != nil { // first service to come up when all services are down
		sm.handleServiceUp(event, baseInterval)
		return
	}

	if event.OldActive != nil { // no service to take over
		sm.handleServiceDown(event)
		return
	}
}

func (sm *ServicesManager) handleNewActiveMember(event *PromotionEvent, baseInterval timesutil.Duration) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	demotedInterval := event.NewActive.ScheduledInterval

	oldActiveGSLBService := event.OldActive.GSLBService()
	oldActiveGSLBService.IsActive = false
	err := sm.svcRepo.Update(oldActiveGSLBService)
	if err != nil {
		bslog.Error("failed to remove active flag from service", slog.Any("oldActive", event.OldActive))
		return
	}

	newActiveGSLBService := event.NewActive.GSLBService()
	newActiveGSLBService.IsActive = true
	err = sm.svcRepo.Update(newActiveGSLBService)
	if err != nil {
		bslog.Error("failed to update active flag on service", slog.Any("newActive", event.NewActive))
		return
	}

	bslog.Warn("demoting service",
		slog.Any("oldActive", event.OldActive),
		slog.Group("intervalChange",
			slog.String("from", event.OldActive.ScheduledInterval.String()),
			slog.String("to", demotedInterval.String()),
		))
	sm.moveServiceToInterval(event.OldActive, demotedInterval)

	sm.DNSUpdate(event.OldActive, false)

	bslog.Warn("promoting service",
		slog.Any("newActive", event.NewActive),
		slog.Group("intervalChange",
			slog.String("from", event.NewActive.ScheduledInterval.String()),
			slog.String("to", baseInterval.String()),
		))
	sm.moveServiceToInterval(event.NewActive, baseInterval)
	sm.DNSUpdate(event.NewActive, true)

	events.Emit(&events.Event{
		Type: domainEvents.EventTypeGSLBServiceFailover,
		Payload: domainEvents.GSLBServiceFailoverEvent{
			MemberOf:   event.Service,
			LastActive: *event.OldActive.GSLBService(),
			NewActive:  *event.NewActive.GSLBService(),
		},
		Timestamp: time.Now(),
		ID:        events.ID(domainEvents.EventTypeGSLBServiceFailover, event.Service),
	})
}

func (sm *ServicesManager) handleServiceUp(event *PromotionEvent, baseInterval timesutil.Duration) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	newActiveGSLBService := event.NewActive.GSLBService()
	newActiveGSLBService.IsActive = true

	err := sm.svcRepo.Update(newActiveGSLBService)
	if err != nil {
		bslog.Error("failed to update active flag on service", slog.Any("newActive", event.NewActive))
		return
	}
	bslog.Info("new gslb service status",
		slog.Any("service", event.NewActive),
		slog.String("status", "up"),
	)
	sm.moveServiceToInterval(event.NewActive, baseInterval)
	sm.DNSUpdate(event.NewActive, true)
	events.Emit(&events.Event{
		Type: domainEvents.EventTypeGSLBServiceUp,
		Payload: domainEvents.GSLBServiceUpEvent{
			NewActive: *event.NewActive.GSLBService(),
		},
		Timestamp: time.Now(),
		ID:        events.ID(domainEvents.EventTypeGSLBServiceUp, event.Service),
	})
}

func (sm *ServicesManager) handleServiceDown(event *PromotionEvent) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	oldActiveGSLBService := event.OldActive.GSLBService()
	oldActiveGSLBService.IsActive = false
	err := sm.svcRepo.Update(oldActiveGSLBService)
	if err != nil {
		bslog.Error("failed to remove active flag from service", slog.Any("oldActive", event.OldActive))
		return
	}
	bslog.Warn("new gslb service status",
		slog.String("serviceGroup", event.Service),
		slog.String("status", "down"),
	)
	sm.DNSUpdate(event.OldActive, false)
	events.Emit(&events.Event{
		Type: domainEvents.EventTypeGSLBServiceDown,
		Payload: domainEvents.GSLBServiceDownEvent{
			MemberOf: event.OldActive.MemberOf,
		},
		Timestamp: time.Now(),
		ID:        events.ID(domainEvents.EventTypeGSLBServiceDown, event.Service),
	})
}

func (sm *ServicesManager) newServiceGroup(memberOf string) *ServiceGroup {
	serviceGroup, ok := sm.serviceGroups[memberOf]
	if ok {
		return serviceGroup
	}
	newGroup := NewEmptyServiceGroup(memberOf)
	newGroup.OnPromotion = func(event *PromotionEvent) {
		sm.handlePromotion(event)
	}
	sm.serviceGroups[memberOf] = newGroup

	serviceGroups.Inc()
	return newGroup
}

// only called when we know it is safe to delete a group
func (sm *ServicesManager) deleteGroup(memberOf string) {
	sm.mutex.Lock()
	delete(sm.serviceGroups, memberOf)
	sm.mutex.Unlock()
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

	gslbService, err := sm.svcRepo.GetMemberInGroup(config.MemberOf, config.ServiceID)
	if err != nil {
		if errors.Is(err, svcRepo.ErrServiceInGroupNotFound) {
			bslog.Debug("could not find member in group",
				slog.String("group", config.MemberOf),
				slog.String("member", config.ServiceID),
			)
		}
		// max out the failure count
		// means a long time before service will be considered healthy
		opts = append(opts, service.WithFailureCount(config.FailureThreshold))

		return opts
	}

	opts = append(opts, service.WithFailureCount(gslbService.FailureCount))
	if gslbService.IsHealthy {
		opts = append(opts, service.WithHealthy())
	}

	return opts
}

func (sm *ServicesManager) ServiceHealthChangeCallback(event *service.HealthChangeEvent) {
	bslog.Debug("received health-change", slog.Any("service", event.Svc), slog.Bool("healthy", event.Healthy))
	err := sm.svcRepo.Update(event.Svc.GSLBService())
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
	sm.serviceGroups[event.Svc.MemberOf].OnServiceHealthChange(event.Svc, event.Healthy)
}
