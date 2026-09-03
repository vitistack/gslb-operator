package brokers

import (
	"context"
	"log/slog"
	"time"

	"github.com/valkey-io/valkey-go"
	status_broker "github.com/vitistack/gslb-operator/internal/brokers/status"
	webhooks_broker "github.com/vitistack/gslb-operator/internal/brokers/webhooks"
	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/internal/repositories/servicegroup"
	"github.com/vitistack/gslb-operator/internal/repositories/status"
	"github.com/vitistack/gslb-operator/pkg/bslog"
	"github.com/vitistack/gslb-operator/pkg/mq/rabbitmq/connection"
	valkeyStore "github.com/vitistack/gslb-operator/pkg/persistence/store/valkey"
)

func Init(ctx context.Context, client valkey.Client, statusRepo *status.StatusRepo, serviceGroupRepo *servicegroup.ServiceGroupRepo) {
	if config.MQ().Enabled() {
		bslog.Debug("mq enabled configuring connection")
		connection.Configure(
			connection.WithLogger(slog.Default()),
			connection.WithRetryBackoff(time.Second*30),
		)

		if config.Webhooks().Enabled() {
			bslog.Debug("webhooks enabled: initializing webhooks broker")
			webhooksStore, err := valkeyStore.NewStore[model.WebHook](client, "gslb:webhooks", time.Minute*30)
			if err != nil {
				bslog.Fatal("failed to create valkey store for gslb webhooks", slog.String("reason", err.Error()))
			}

			webhooks_broker.Init(ctx, webhooksStore)
		}

		if config.GSLB().StatusEnabled() {
			bslog.Debug("gslb site status enabled: initializing status broker")
			status_broker.Init(ctx, statusRepo, serviceGroupRepo)
		}
	}
}
