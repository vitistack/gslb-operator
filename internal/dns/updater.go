package dns

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/pkg/auth/jwt"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/request/client"
)

type updaterOption func(u *Updater)

type Updater struct {
	Server  string
	client  client.HTTPClient
	builder *request.Builder
	mu      *sync.Mutex
}

func NewUpdater(opts ...updaterOption) (*Updater, error) {
	c, err := client.NewClient(
		time.Second*5,
		client.WithRetry(3),
		client.WithRequestLogging(slog.Default()),
	)

	if err != nil {
		return nil, fmt.Errorf("unable to create http client: %s", err.Error())
	}

	u := &Updater{
		Server: config.GetInstance().GSLB().UpdaterHost(),
		client: *c,
		mu:     &sync.Mutex{},
	}

	for _, opt := range opts {
		opt(u)
	}
	u.builder = request.NewBuilder(u.Server).SetHeader("User-Agent", config.GetInstance().JWT().User())

	return u, nil
}

func UpdaterWithServer(server string) updaterOption {
	return func(u *Updater) {
		u.Server = server
	}
}

func UpdaterWithClient(client *client.HTTPClient) updaterOption {
	return func(u *Updater) {
		u.client = *client
	}
}

func (u *Updater) ServiceDown(svc *service.Service) error {
	token, err := jwt.GetInstance().GetServiceToken()
	if err != nil {
		return fmt.Errorf("could not fetch service token: %w", err)
	}

	req, err := u.builder.DELETE().
		SetHeader("Authorization", token).
		URL(fmt.Sprintf("/spoofs/%s:%s", svc.MemberOf, svc.Datacenter)).
		Build()
	if err != nil {
		return fmt.Errorf("could not create delete request for update: %s", err.Error())
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("spoof deletion on service down failed: %s", err.Error())
	}

	if !(resp.StatusCode >= 200 && resp.StatusCode <= 299) {
		return fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	return nil
}

func (u *Updater) ServiceUp(svc *service.Service) error {
	token, err := jwt.GetInstance().GetServiceToken()
	if err != nil {
		return fmt.Errorf("could not fetch service token: %w", err)
	}

	req, err := u.builder.POST().SetHeader("Authorization", token).
		URL("/spoofs").
		Body(spoofs.Spoof{
			FQDN: svc.MemberOf,
			IP:   svc.GetIP(),
			DC:   svc.Datacenter,
		}).
		Build()
	if err != nil {
		return fmt.Errorf("could not create post request for update: %s", err.Error())
	}

	_, err = u.client.Do(req)
	if err != nil {
		return fmt.Errorf("request for update failed: %s", err.Error())
	}

	return nil
}
