package spoofs

import (
	"fmt"

	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
)

type Client interface {
	Get() error
}

type SpoofsClient struct {
	transport transport.Transport
	path string
}

func NewSpoofClient(transport transport.Transport, memberOf string) *SpoofsClient {
	return &SpoofsClient{
		transport: transport,
		path: fmt.Sprintf("/service/%s/spoofs", memberOf),
	}
}

func (c *SpoofsClient) Get() error {
	return nil
}
