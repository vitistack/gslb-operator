package status_broker

import (
	"context"
	"fmt"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/repositories/status"
	serviceModels "github.com/vitistack/gslb-operator/pkg/models/service"
	"github.com/vitistack/gslb-operator/pkg/mq"
	"github.com/vitistack/gslb-operator/pkg/mq/rabbitmq"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

// periodically
type StatusBroker struct {
	client     mq.MessageBroker[serviceModels.SiteGSLBServiceStatus]
	statusRepo *status.StatusRepo
}

func Init(ctx context.Context, store persistence.Store[serviceModels.GSLBServiceStatus])

func NewStatusBroker(ctx context.Context, store persistence.Store[serviceModels.GSLBServiceStatus]) *StatusBroker {
	mqCfg := config.MQ()
	broker := &StatusBroker{
		statusRepo: status.NewStatusRepo(store),
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
			//rabbitmq.WithRetryConnectionBackOff[serviceModels.SiteGSLBServiceStatus](time.Second*10),
		),
	}

	return broker
}
