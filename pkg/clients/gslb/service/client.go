package service

import (
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/service/override"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/service/spoofs"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/service/status"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
)

type Client interface {
	Status() status.Client
	Spoofs() spoofs.Client
	Override() override.Client
}

type ServiceClient struct {
	memberOf  string
	transport transport.Transport
}

func NewServiceClient(transport transport.Transport, memberOf string) *ServiceClient {
	return &ServiceClient{
		transport: transport,
		memberOf:  memberOf,
	}
}

func (c *ServiceClient) Status() status.Client {
	return status.NewStatusClient(c.transport, c.memberOf)
}

func (c *ServiceClient) Spoofs() spoofs.Client {
	return spoofs.NewSpoofClient(c.transport, c.memberOf)
}

func (c *ServiceClient) Override() override.Client {
	return override.NewOverrideClient(c.transport, c.memberOf)
}
