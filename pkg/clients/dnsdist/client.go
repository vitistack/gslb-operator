package dnsdist

import (
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/pools"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/rules"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/servers"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/stats"
	"github.com/vitistack/gslb-operator/pkg/clients/dnsdist/transport"
)

type Client interface {
	Pools() pools.Client
	Rules() rules.Client
	Servers() servers.Client
	Stats() stats.Client
	Close() error
}

type dnsDistClient struct {
	pools     pools.Client
	rules     rules.Client
	servers   servers.Client
	stats     stats.Client
	transport transport.Transport
}

func NewClient(transport transport.Transport) Client {
	return &dnsDistClient{
		rules:     rules.New(transport),
		transport: transport,
	}
}

func (c *dnsDistClient) Pools() pools.Client {
	return c.pools
}

func (c *dnsDistClient) Rules() rules.Client {
	return c.rules
}

func (c *dnsDistClient) Servers() servers.Client {
	return c.servers
}

func (c *dnsDistClient) Stats() stats.Client {
	return c.stats
}

func (c *dnsDistClient) Close() error {
	return c.transport.Close()
}
