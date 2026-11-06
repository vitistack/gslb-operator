package dnsdist

import (
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/dnsdist/config"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
)

type Client interface {
	Config() config.Client
}

type dnsdistClient struct {
	transport transport.Transport
}

func NewDNSDISTClient(transport transport.Transport) *dnsdistClient {
	return &dnsdistClient{
		transport: transport,
	}
}

func (c *dnsdistClient) Config() config.Client {
	return config.NewConfigClient(c.transport)
}
