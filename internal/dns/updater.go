package dns

import (
	"fmt"
	"log/slog"
	"sync"
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
	mu        *sync.Mutex
	getActive func(string) *service.Service
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
		mu:        &sync.Mutex{},
	}

	for _, opt := range opts {
		opt(u)
	}
	u.builder = request.NewBuilder(u.Server).SetHeader("User-Agent", config.GetInstance().JWT().User())

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
	u.mu.Lock()
	err := u.spoofRepo.Delete(fmt.Sprintf("%s:%s", svc.MemberOf, svc.Datacenter))
	u.mu.Unlock()
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
	u.mu.Lock()
	err = u.spoofRepo.Create(fmt.Sprintf("%s:%s", svc.MemberOf, svc.Datacenter), spoof)
	u.mu.Unlock()
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

func (u *Updater) CreateOverride(spoof spoofs.Spoof) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	exist, err := u.spoofRepo.Read(spoof.Name)
	if err != nil {
		return fmt.Errorf("unable to read spoofs from storage: %w", err)
	}

	if exist.Name != spoof.Name {
		return fmt.Errorf("spoof with name: %s does not exist", spoof.Name)
	}

	exist.Override = true
	err = u.spoofRepo.Update(exist.Name, &exist)
	if err != nil {
		return fmt.Errorf("could not update spoof: %w", err)
	}

	/* 
	* TODO:
	* should update be done to GSLB - updater?
	* or should sync-job take care of it?
	*/

	return nil
}

func (u *Updater) DeleteOverride(spoof spoofs.Spoof) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	exist, err := u.spoofRepo.Read(spoof.Name)
	if err != nil {
		return fmt.Errorf("unable to read spoofs from storage: %w", err)
	}

	if exist.Name != spoof.Name {
		return fmt.Errorf("spoof with name: %s does not exist", spoof.Name)
	}

	// restore the spoof with the correct service
	svc := u.getActive(spoof.Name)
	if svc == nil {
		err := u.spoofRepo.Delete(spoof.Name)
		if err != nil {
			return fmt.Errorf("unable to delete spoof, due to service ")
		}
		return nil
	}

	exist.Override = false
	exist.Name = svc.MemberOf + ":" + svc.Datacenter
	exist.FQDN = svc.Fqdn
	exist.IP, _ = svc.GetIP()
	exist.DC = svc.Datacenter

	err = u.spoofRepo.Update(exist.Name, &exist)
	if err != nil {
		return fmt.Errorf("unable to update spoof after override deletion: override still active: %w", err)
	}

	return nil
}
