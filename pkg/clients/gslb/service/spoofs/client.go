package spoofs

import (
	"context"
	"fmt"

	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
)

type Client interface {
	Get(context.Context) error
}

type SpoofsClient struct {
	transport transport.Transport
	path      string
}

func NewSpoofClient(transport transport.Transport, basePath string) *SpoofsClient {
	return &SpoofsClient{
		transport: transport,
		path:      fmt.Sprintf("%s/spoofs", basePath),
	}
}

func (c *SpoofsClient) Get(context.Context) error {
	return nil
}
