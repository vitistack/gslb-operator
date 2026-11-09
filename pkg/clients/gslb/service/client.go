package service

import (
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/service/members"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/service/override"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/service/spoofs"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/service/status"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
)

type Client interface {
	Status() status.Client
	Spoofs() spoofs.Client
	Override() override.Client
	Members() members.Client
}

type serviceClient struct {
	transport transport.Transport
	path      string
}

func NewServiceClient(transport transport.Transport, memberOf string) *serviceClient {
	return &serviceClient{
		transport: transport,
		path:      "/service/" + memberOf,
	}
}

func (c *serviceClient) Status() status.Client {
	return status.NewStatusClient(c.transport, c.path)
}

func (c *serviceClient) Spoofs() spoofs.Client {
	return spoofs.NewSpoofClient(c.transport, c.path)
}

func (c *serviceClient) Override() override.Client {
	return override.NewOverrideClient(c.transport, c.path)
}

func (c *serviceClient) Members() members.Client {
	return members.NewMembersClient(c.transport, c.path)
}
