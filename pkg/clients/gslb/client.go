package gslb

import (
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/auth"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/dnsdist"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/services"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
)

type Client interface {
	Auth() auth.Client
	Service(memberOf string) services.Client
	DNSDIST() dnsdist.Client
}

type client struct {
	authClient auth.Client
	transport  transport.Transport
}

func NewClient(transport transport.Transport) *client {
	return &client{
		authClient: auth.NewAuthClient(transport),
		transport:  transport,
	}
}

func (c *client) Auth() auth.Client {
	return c.authClient
}

func (c *client) Services(memberOf ...string) services.Client {
	return services.NewServiceClient(c.transport, memberOf...)
}

func (c *client) DNSDIST() dnsdist.Client {
	return dnsdist.NewDNSDISTClient(c.transport)
}
