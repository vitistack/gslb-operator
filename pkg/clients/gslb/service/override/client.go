package override

import (
	"context"
	"fmt"

	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
)

type Client interface {
	Create(context.Context) error
	Get(context.Context) error
	Update(context.Context) error
	Delete(context.Context) error
}

type OverrideClient struct {
	transport transport.Transport
	path      string
}

func NewOverrideClient(transport transport.Transport, memberOf string) *OverrideClient {
	return &OverrideClient{
		transport: transport,
		path:      fmt.Sprintf("/service/%s/override", memberOf),
	}
}

func (c *OverrideClient) Create(context.Context) error {
	return nil
}

func (c *OverrideClient) Get(context.Context) error {
	return nil
}

func (c *OverrideClient) Update(context.Context) error {
	return nil
}

func (c *OverrideClient) Delete(context.Context) error {
	return nil
}
