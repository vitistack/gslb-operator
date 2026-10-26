package gslb

import (
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/auth"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/service"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
)

type Client interface {
	Auth() auth.Client
	Service(memberOf string) service.Client
}

type client struct {
	authClient    auth.Client
	serviceClient *service.ServiceClient
	transport     transport.Transport
}

func NewClient(transport transport.Transport) Client {
	return &client{
		authClient: auth.NewAuthClient(transport),
		transport:  transport,
	}
}

func (c *client) Auth() auth.Client {
	return c.authClient
}

func (c *client) Service(memberOf string) service.Client {
	return service.NewServiceClient(c.transport, memberOf)
}
