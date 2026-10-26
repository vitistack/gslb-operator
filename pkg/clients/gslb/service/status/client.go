package status

import (
	"context"
	"fmt"

	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
	serviceModels "github.com/vitistack/gslb-operator/pkg/models/service"
)

type Client interface {
	Get(context.Context) (serviceModels.GSLBServiceStatus, error)
}

type StatusClient struct {
	transport transport.Transport
	path      string
}

func NewStatusClient(transport transport.Transport, memberOf string) *StatusClient {
	return &StatusClient{
		transport: transport,
		path:      fmt.Sprintf("/service/%s/status", memberOf),
	}
}

func (c *StatusClient) Get(ctx context.Context) (serviceModels.GSLBServiceStatus, error) {
	var status serviceModels.GSLBServiceStatus
	err := c.transport.GetJSON(ctx, c.path, &status)
	if err != nil {
		return serviceModels.GSLBServiceStatus{}, err
	}

	return status, nil
}
