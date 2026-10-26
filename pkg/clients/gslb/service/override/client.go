package override

import (
	"fmt"

	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
)

type Client interface {
	Create() error
	Get() error
	Update() error
	Delete() error
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

func (c *OverrideClient) Create() error {
	return nil
}

func (c *OverrideClient) Get() error {
	return nil
}

func (c *OverrideClient) Update() error {
	return nil
}

func (c *OverrideClient) Delete() error {
	return nil
}
