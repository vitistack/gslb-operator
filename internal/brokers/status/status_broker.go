package status_broker

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/vitistack/gslb-operator/internal/config"
	domainEvents "github.com/vitistack/gslb-operator/internal/model/events"
	"github.com/vitistack/gslb-operator/internal/repositories/servicegroup"
	"github.com/vitistack/gslb-operator/internal/repositories/status"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/events"
	"github.com/vitistack/gslb-operator/pkg/models/service"
	serviceModels "github.com/vitistack/gslb-operator/pkg/models/service"
	"github.com/vitistack/gslb-operator/pkg/mq"
	"github.com/vitistack/gslb-operator/pkg/mq/rabbitmq"
)

// periodically
type StatusBroker struct {
	client           mq.MessageBroker[serviceModels.SiteGSLBServiceStatus]
	statusRepo       *status.StatusRepo
	serviceGroupRepo *servicegroup.ServiceGroupRepo
}

func Init(ctx context.Context, repo *status.StatusRepo) {
	if config.GSLB().StatusEnabled() {
		NewStatusBroker(ctx, repo).Subscribe(ctx)
	}
}

func NewStatusBroker(ctx context.Context, repo *status.StatusRepo) *StatusBroker {
	mqCfg := config.MQ()
	broker := &StatusBroker{
		statusRepo: repo,
		client: rabbitmq.New(
			ctx,
			fmt.Sprintf(
				"amqp://%s:%s@%s:%s",
				mqCfg.User(),
				mqCfg.Pass(),
				mqCfg.Endpoint(),
				mqCfg.Port(),
			),
			rabbitmq.WithQueue[serviceModels.SiteGSLBServiceStatus]("q.gslb.service-status"),
		),
	}

	events.On(domainEvents.EventTypeGSLBService, broker)

	return broker
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
					bslog.Error("webhooks subscription stopped unexpectedly")
				}
			} else {
				select {
				case <-ctx.Done():
					return
				default:
					bslog.Error("webhooks subscription failed", slog.String("reason", err.Error()))
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
		bslog.Debug("skipping status event handle due to invalid type: %T", e.Payload)
		return
	}

	siteStatus := service.SiteGSLBServiceStatus{
		Service: event.GetMemberOf(),
		Site:    config.GSLB().Site(),
		LocalGSLBServiceStatus: serviceModels.LocalGSLBServiceStatus{
			LastSeen: e.Timestamp,
			Members:  make([]serviceModels.ShortGSLBServiceMemberStatus, 0),
		},
	}

	serviceGroup, err := s.serviceGroupRepo.Read(event.GetMemberOf())
	if err != nil {
		bslog.Error(
			"failed to publish gslb-site status for "+event.GetMemberOf(),
			slog.String("reason", err.Error()),
		)
	}

	if len(serviceGroup.Active) == 0 {
		siteStatus.Healthy = false
	} else {
		siteStatus.Healthy = true
	}

	for _, member := range serviceGroup.Members {
		siteStatus.LocalGSLBServiceStatus.Members = append(siteStatus.LocalGSLBServiceStatus.Members, member.GSLBServiceMemberStatus())
	}
	
	

	err = s.Publish(context.Background(), siteStatus)
	if err != nil {
		bslog.Error(
			"failed to publish gslb-site status for "+event.GetMemberOf(),
			slog.String("reason", err.Error()),
		)
	}
}

// interface satisfaction for EventHandler on internal events.Emit(...)
func (s *StatusBroker) GetID() string {
	return ""
}

func (s *StatusBroker) handleSiteStatus(ctx context.Context, status serviceModels.SiteGSLBServiceStatus) error {
	gslbStatus, err := s.statusRepo.Read(status.Service)
	if err != nil {
		return fmt.Errorf("failed to read gslb status: %w", err)
	}

	idx := slices.IndexFunc(
		gslbStatus.Sites,
		func(s serviceModels.SiteGSLBServiceStatus) bool { return s.Site == status.Site },
	)
	if idx == -1 {
		gslbStatus.Sites = append(gslbStatus.Sites, status)
		return s.statusRepo.Update(gslbStatus.MemberOf, gslbStatus)
	}

	gslbStatus.Sites[idx] = status

	return s.statusRepo.Update(gslbStatus.MemberOf, gslbStatus)
}
