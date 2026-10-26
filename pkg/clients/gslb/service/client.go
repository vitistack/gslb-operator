package service

import (
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/service/status"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
)

type Client interface {
	Status() status.Client
}

type ServiceClient struct {
	statusClient status.Client
	memberOf     string
	transport    transport.Transport
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
