package manager

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/vitistack/gslb-operator/internal/manager/scheduler"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/internal/utils/timesutil"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/pool"
)

// Responsible for managing services, on scheduling services for health checks
type ServicesManager struct {
	// servicesHealthCheck maps check intervals to services that should be checked at that interval.
	scheduledServices ScheduledServices                           // services that are scheduled on an interval
	schedulers        map[timesutil.Duration]*scheduler.Scheduler // schedulers for health-checks
	serviceGroups     map[string]*ServiceGroup
	mutex             sync.RWMutex
	stop              sync.Once
	pool              pool.WorkerPool
	wg                sync.WaitGroup
	DNSUpdate         func(*service.Service, bool)
	dryrun            bool
}

func NewManager(opts ...serviceManagerOption) *ServicesManager {
	cfg := managerConfig{
		MinRunningWorkers:     100,
		NonBlockingBufferSize: 110,
		DryRun:                false,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.DryRun {
		bslog.Warn("dry-run enabled")
	}

	return &ServicesManager{
		scheduledServices: make(ScheduledServices),
		schedulers:        make(map[timesutil.Duration]*scheduler.Scheduler),
		serviceGroups:     make(map[string]*ServiceGroup),
		mutex:             sync.RWMutex{},
		pool:              *pool.NewWorkerPool(cfg.MinRunningWorkers, cfg.NonBlockingBufferSize),
		stop:              sync.Once{},
		wg:                sync.WaitGroup{},
		dryrun:            cfg.DryRun,
	}
}

// Start begins scheduling health checks for all services according to their configured intervals.
// It ensures that the scheduling logic is only executed once, even if called multiple times.
func (sm *ServicesManager) Start() {
	sm.pool.Start()
}

func (sm *ServicesManager) Stop() {
	sm.pool.Stop()
	sm.stop.Do(func() {

		for interval, scheduler := range sm.schedulers {
			scheduler.Stop()
			bslog.Debug("scheduler closed", slog.String("interval", interval.String()))
		}

		sm.wg.Wait()
		bslog.Debug("service manager closed")
	})
}

func (sm *ServicesManager) RegisterService(serviceCfg model.GSLBConfig) (*service.Service, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	newService, err := service.NewServiceFromGSLBConfig(serviceCfg, sm.dryrun) // create the service object
	if err != nil {
		return nil, fmt.Errorf("unable to register service: %s", err.Error())
	}

	_, oldSvc := sm.scheduledServices.Search(newService.GetID(), newService.ScheduledInterval)
	if oldSvc != nil { // update service if already exists
		sm.updateServiceUnlocked(oldSvc, newService)
		return newService, nil
	}

	// set healthchange callback action
	newService.SetHealthChangeCallback(func(healthy bool) {
		bslog.Debug("received health-change", slog.Any("service", newService), slog.Bool("healthy", healthy))
		sm.serviceGroups[newService.MemberOf].OnServiceHealthChange(newService, healthy)
	})

	// create new scheduler if needed, and schedule service for health-checks
	scheduler := sm.newScheduler(newService.ScheduledInterval)
	scheduler.ScheduleService(newService)

	// register the service in the datastructure
	sm.scheduledServices.Add(newService)

	// create new service group if needed, and register service in group
	memberOf := newService.MemberOf
	serviceGroup, ok := sm.serviceGroups[memberOf]
	if !ok {
		serviceGroup = sm.newServiceGroup(memberOf)
		bslog.Debug("new service group", slog.String("group", newService.MemberOf))
	}
	serviceGroup.RegisterService(newService)

	bslog.Debug("registered service", slog.Any("service", newService))
	return newService, nil
}

// removes the service from its healthcheck queue
func (sm *ServicesManager) RemoveService(service *service.Service) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	_, svc := sm.scheduledServices.Search(service.GetID(), service.ScheduledInterval)
	if svc == nil { // cannot remove something that does not exists
		return ErrServiceNotFound
	}

	sm.schedulers[service.ScheduledInterval].RemoveService(service) // remove the service from its scheduler

	group := sm.serviceGroups[service.MemberOf]
	empty := group.RemoveService(service) // registered in group
	if empty {
		delete(sm.serviceGroups, service.MemberOf)
	}
	sm.scheduledServices.Delete(service.GetID(), service.ScheduledInterval)
	bslog.Debug("removed service", slog.Any("service", service))

	return nil
}

// updates an existing service with new configuration
// assumes sm.mutex is held by the caller
func (sm *ServicesManager) updateServiceUnlocked(old, new *service.Service) {
	if old == new {
		return
	}

	if !old.ConfigChanged(new) { // nothing to do
		return
	}

	oldDefaultInterval, newDefaultInterval := old.GetDefaultInterval(), new.GetDefaultInterval()
	if oldDefaultInterval != newDefaultInterval && oldDefaultInterval == old.ScheduledInterval {
		// we need to move the service to a new interval
		// otherwise the service will get rescheduled back to its default interval on its own, when it is needed
		sm.moveServiceToInterval(old, newDefaultInterval)
	}

	if old.MemberOf != new.MemberOf {
		oldGroup := sm.serviceGroups[old.MemberOf]
		oldGroup.RemoveService(old)

		if len(oldGroup.Members) == 0 {
			// TODO: delete service group
			bslog.Error("TODO: delete service group")
		}
	}

	old.Assign(new)

	bslog.Debug("updated service", slog.Any("service", old))
}

// re-schedules the relevant services in the PromotionEvent
func (sm *ServicesManager) handlePromotion(event *PromotionEvent) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

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
	var demotedInterval timesutil.Duration

	msg := "received promotion event for service: " + event.Service + ": "
	if event.OldActive != nil { // set baseInterval
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

	if event.OldActive != nil && event.NewActive != nil { // just swap, and do dns updates
		demotedInterval = event.NewActive.ScheduledInterval

		bslog.Info("demoting service",
			slog.Any("oldActive", event.OldActive),
			slog.Group("intervalChange",
				slog.String("from", event.OldActive.ScheduledInterval.String()),
				slog.String("to", demotedInterval.String()),
			))
		sm.moveServiceToInterval(event.OldActive, demotedInterval)
		sm.DNSUpdate(event.OldActive, false)

		bslog.Info("promoting service",
			slog.Any("newActive", event.NewActive),
			slog.Group("intervalChange",
				slog.String("from", event.NewActive.ScheduledInterval.String()),
				slog.String("to", baseInterval.String()),
			))
		sm.moveServiceToInterval(event.NewActive, baseInterval)
		sm.DNSUpdate(event.NewActive, true)
		return
	}

	if event.NewActive != nil { // first service to come up when all services are down
		bslog.Info("new active service", slog.Any("service", event.NewActive))
		sm.moveServiceToInterval(event.NewActive, baseInterval)
		sm.DNSUpdate(event.NewActive, true)
		return
	}

	if event.OldActive != nil { // no service to take over
		bslog.Warn("no available sites", slog.String("serviceGroup", event.Service))
		sm.DNSUpdate(event.OldActive, false)
		return
	}
}

func (sm *ServicesManager) newServiceGroup(memberOf string) *ServiceGroup {
	sm.serviceGroups[memberOf] = new(ServiceGroup)
	newGroup := NewEmptyServiceGroup()

	newGroup.OnPromotion = func(event *PromotionEvent) {
		sm.handlePromotion(event)
	}
	sm.serviceGroups[memberOf] = newGroup

	return newGroup
}

// creates a new scheduler, and starts its loop
func (sm *ServicesManager) newScheduler(interval timesutil.Duration) *scheduler.Scheduler {
	if scheduler, ok := sm.schedulers[interval]; ok { // scheduler already exists
		return scheduler
	}

	scheduler := scheduler.NewScheduler(time.Duration(interval))
	sm.schedulers[interval] = scheduler

	scheduler.OnTick = func(s *service.Service) {
		err := sm.pool.Put(s)
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
	newScheduler.ScheduleService(svc)
}

func (sm *ServicesManager) GetActiveForMemberOf(memberOf string) *service.Service {
	if group, ok := sm.serviceGroups[memberOf]; ok {
		return group.GetActive()
	}
	return nil
}
