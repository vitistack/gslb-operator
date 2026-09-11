package status_broker

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/dns"
	"github.com/vitistack/gslb-operator/internal/model"
	domainEvents "github.com/vitistack/gslb-operator/internal/model/events"
	"github.com/vitistack/gslb-operator/internal/repositories/servicegroup"
	"github.com/vitistack/gslb-operator/internal/repositories/status"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/models/service"
	serviceModels "github.com/vitistack/gslb-operator/pkg/models/service"
	"github.com/vitistack/gslb-operator/pkg/mq"
	"github.com/vitistack/gslb-operator/pkg/mq/rabbitmq"
	"github.com/vitistack/gslb-operator/pkg/scheduler"
)

// periodically
type StatusBroker struct {
	client           mq.MessageBroker[serviceModels.SiteGSLBServiceStatus]
	dnsStatusFetcher dns.StatusFetcher
	statusRepo       *status.StatusRepo
	serviceGroupRepo *servicegroup.ServiceGroupRepo
	scheduler        *scheduler.Scheduler[string]
	wg               *sync.WaitGroup
}

func Init(ctx context.Context, statusRepo *status.StatusRepo, groupRepo *servicegroup.ServiceGroupRepo, dnsStatusFetcher dns.StatusFetcher) {
	if config.GSLB().StatusEnabled() {
		NewStatusBroker(ctx, statusRepo, groupRepo, dnsStatusFetcher).Subscribe(ctx)
	}
}

func NewStatusBroker(ctx context.Context, statusRepo *status.StatusRepo, groupRepo *servicegroup.ServiceGroupRepo, dnsStatusFetcher dns.StatusFetcher) *StatusBroker {
	mqCfg := config.MQ()
	var amqpUrlPrefix string

	switch config.Server().Environment() {
	case "local", "LOCAL":
		amqpUrlPrefix = "amqp://"
	default:
		amqpUrlPrefix = "amqps://"
	}
	wg := sync.WaitGroup{}
	scheduler := scheduler.NewScheduler[string](time.Minute*5, &wg)

	broker := &StatusBroker{
		statusRepo:       statusRepo,
		serviceGroupRepo: groupRepo,
		dnsStatusFetcher: dnsStatusFetcher,
		scheduler:        &scheduler,
		wg:               &wg,
		client: rabbitmq.New(
			ctx,
			fmt.Sprintf(
				"%s%s:%s@%s",
				amqpUrlPrefix,
				mqCfg.User(),
				mqCfg.Pass(),
				mqCfg.Endpoint(),
			),
			rabbitmq.WithExchange[serviceModels.SiteGSLBServiceStatus]("ex.gslb.service-status"),
			rabbitmq.WithQueue[serviceModels.SiteGSLBServiceStatus]("q.gslb.service-status"),
			rabbitmq.WithFanout[serviceModels.SiteGSLBServiceStatus](),
		),
	}
	broker.scheduler.OnTick(func(memberOf string) {
		go func() {
			broker.PublishStatusForGroup(memberOf, time.Now())
		}()
	})

	events.On(domainEvents.EventTypeGSLBService, broker)
	broker.Subscribe(ctx)

	go broker.coldStart()
	go func() {
		<-ctx.Done()
		broker.scheduler.Stop()
	}()

	return broker
}

func (s *StatusBroker) coldStart() {
	counter := 0
	groups, finish := s.serviceGroupRepo.ReadAll()

	groups.Each(
		func(group model.GSLBServiceGroup) {
			var memberOf string
			for _, member := range group.Members {
				memberOf = member.MemberOf
				break
			}

			if memberOf != "" {
				counter++
				bslog.Debug("scheduling regular service status publishing", slog.String("memberOf", memberOf))
				s.scheduler.Schedule(memberOf)
			}
		},
	)

	bslog.Debug("scheduled regular service status updates", slog.Int("servicesTotal", counter))
	if err := finish(); err != nil {
		bslog.Error("failed to read service groups for status scheduling", slog.String("reason", err.Error()))
	}
}

func (s *StatusBroker) reconcileScheduledGroup(memberOf string) {
	group, err := s.serviceGroupRepo.Read(memberOf)
	if err != nil || len(group.Members) == 0 {
		s.scheduler.Remove(func(m string) bool { return m == memberOf })
	}
}

func (s *StatusBroker) Publish(ctx context.Context, siteStatus serviceModels.SiteGSLBServiceStatus) error {
	err := s.client.Publish(ctx, siteStatus)
	if err != nil {
		return fmt.Errorf("failed to publish gslb-service status for site: %s: %w", siteStatus.Site, err)
	}

	return nil
}

func (s *StatusBroker) Subscribe(ctx context.Context) {
	go func() {
		const retryDelay = time.Second * 5

		for {
			err := s.client.Subscribe(ctx, s.handleSiteStatus)
			if err == nil {
				select {
				case <-ctx.Done():
					return
				default:
					bslog.Error("status subscription stopped unexpectedly")
				}
			} else {
				select {
				case <-ctx.Done():
					return
				default:
					bslog.Error("status subscription failed", slog.String("reason", err.Error()))
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDelay):
			}
		}
	}()
}

func (s *StatusBroker) Handle(e *events.Event) {
	event, ok := e.Payload.(domainEvents.GSLBServiceEvent)
	if !ok {
		bslog.Debug(fmt.Sprintf("skipping status event handle due to invalid type: %T", e.Payload))
		return
	}

	bslog.Debug("handling status-update event", slog.String("eventType", fmt.Sprintf("%T", e.Payload)))

	switch e.Payload.(type) {
	case domainEvents.GSLBServiceMemberAddEvent, domainEvents.GSLBServiceMemberRemoveEvent:
		s.reconcileScheduledGroup(event.GetMemberOf())
	}

	if err := s.PublishStatusForGroup(event.GetMemberOf(), e.Timestamp); err != nil {
		bslog.Error(
			"failed to publish gslb-site status for "+event.GetMemberOf(),
			slog.String("reason", err.Error()),
			slog.String("event_id", e.ID),
		)
	}
}

// interface satisfaction for EventHandler on internal events.Emit(...)
func (s *StatusBroker) GetID() string {
	return "status:broker"
}

func (s *StatusBroker) handleSiteStatus(ctx context.Context, status serviceModels.SiteGSLBServiceStatus) error {
	bslog.Debug("received gslb site status", slog.Any("status", status))
	gslbStatus, err := s.statusRepo.Read(status.MemberOf)
	if err != nil {
		bslog.Error("failed to read gslb status", slog.String("reason", err.Error()))
		return fmt.Errorf("failed to read gslb status: %w", err)
	}

	if gslbStatus.MemberOf == "" {
		gslbStatus.MemberOf = status.MemberOf
	}

	idx := slices.IndexFunc(
		gslbStatus.Sites,
		func(s serviceModels.SiteStatus) bool { return s.Site == status.Site },
	)
	if idx == -1 {
		gslbStatus.Sites = append(gslbStatus.Sites, status.SiteStatus)
		return s.statusRepo.Update(status.MemberOf, gslbStatus)
	}

	gslbStatus.Sites[idx] = status.SiteStatus

	return s.statusRepo.Update(status.MemberOf, gslbStatus)
}

func (s *StatusBroker) PublishStatusForGroup(group string, lastSeen time.Time) error {
	siteStatus := service.SiteGSLBServiceStatus{
		MemberOf: group,
		Site:     config.GSLB().Site(),
		LastSeen: lastSeen,
		Members:  make([]serviceModels.ShortGSLBServiceMemberStatus, 0),
	}

	serviceGroup, err := s.serviceGroupRepo.Read(group)
	if err != nil {
		return err
	}

	siteStatus.Health = service.HEALTHY
	numUnhealthy := 0

	for _, member := range serviceGroup.Members {
		if !member.IsHealthy {
			siteStatus.Health = service.DEGRADED
			numUnhealthy++
		}
		siteStatus.LocalGSLBServiceStatus.Members = append(siteStatus.LocalGSLBServiceStatus.Members, member.GSLBServiceMemberStatus())
	}

	if numUnhealthy >= len(siteStatus.Members) {
		siteStatus.Health = service.DOWN
	}

	dnsStatus, err := s.dnsStatusFetcher.FetchStatus(serviceGroup.UUID.String())
	if err != nil {
		return err
	}
	siteStatus.DNSStatus = dnsStatus

	err = s.Publish(context.Background(), siteStatus)
	if err != nil {
		return err
	}

	return nil
}
