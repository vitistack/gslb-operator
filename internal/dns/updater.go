package dns

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/repositories/spoof"
	"github.com/vitistack/gslb-operator/internal/service"
	"github.com/vitistack/gslb-operator/pkg/auth/jwt"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence/store/memory"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
	"github.com/vitistack/gslb-operator/pkg/rest/request/client"
)

type updaterOption func(u *Updater)

type Updater struct {
	Server    string
	spoofRepo *spoof.Repository
	client    client.HTTPClient
	builder   *request.Builder
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
		Server:    config.GetInstance().GSLB().UpdaterHost(),
		spoofRepo: spoof.NewRepository(memory.NewStore[spoofs.Spoof]()),
		client:    *c,
	}

	for _, opt := range opts {
		opt(u)
	}
	u.builder = request.NewBuilder(u.Server)

	return u, nil
}

func UpdaterWithSpoofRepo(spoofRep *spoof.Repository) updaterOption {
	return func(u *Updater) {
		u.spoofRepo = spoofRep
	}
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
	err := u.spoofRepo.Delete(fmt.Sprintf("%s:%s", svc.MemberOf, svc.Datacenter))
	if err != nil {
		return fmt.Errorf("unable to delete service from storage: %s", err.Error())
	}

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
	ip, err := svc.GetIP()
	if err != nil {
		return fmt.Errorf("unable to get ip address: %s", err.Error())
	}

	spoof := &spoofs.Spoof{
		FQDN: svc.MemberOf,
		IP:   ip,
		DC:   svc.Datacenter,
	}
	err = u.spoofRepo.Create(fmt.Sprintf("%s:%s", svc.Fqdn, svc.Datacenter), spoof)
	if err != nil {
		return fmt.Errorf("could not store new spoof: %s", err.Error())
	}
	
	token, err := jwt.GetInstance().GetServiceToken()
	if err != nil {
		return fmt.Errorf("could not fetch service token: %w", err)
	}

	req, err := u.builder.POST().SetHeader("Authorization", token).
		URL("/spoofs").
		Body(spoof).
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
