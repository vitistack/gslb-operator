package config

import (
	"context"
	"strings"

	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
	"github.com/vitistack/gslb-operator/pkg/models/dnsdist"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
)

type Client interface {
	Spoofs(context.Context, ...string) ([]spoofs.Spoof, error)
	Lua(context.Context, ...string) (string, error)
}

type configClient struct {
	transport transport.Transport
}

func NewConfigClient(transport transport.Transport) *configClient {
	return &configClient{
		transport: transport,
	}
}

func (c *configClient) Spoofs(ctx context.Context, views ...string) ([]spoofs.Spoof, error) {
	var result []spoofs.Spoof
	if err := c.transport.GetJSON(ctx, path("json", views...), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *configClient) Lua(ctx context.Context, views ...string) (string, error) {
	return c.transport.GetText(ctx, path("lua", views...))
}

func path(format string, views ...string) string {
	params := dnsdist.GetConfigParams{Format: format, Views: strings.Join(views, ",")}
	return "/dnsdist/config?" + request.MarshallParams(&params).Encode()
}
