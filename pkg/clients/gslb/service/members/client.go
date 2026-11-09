package members

import (
	"context"
	"fmt"

	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
	"github.com/vitistack/gslb-operator/pkg/models/pagination"
	"github.com/vitistack/gslb-operator/pkg/models/service"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
)

type Client interface {
	Get(ctx context.Context, params *service.GetMembersParams) (pagination.Pagination[service.Member], error)
	GetID(ctx context.Context, id string) (service.Member, error)
}

type membersClient struct {
	transport transport.Transport
	path      string
}

func NewMembersClient(transport transport.Transport, basePath string) *membersClient {
	return &membersClient{
		transport: transport,
		path:      fmt.Sprintf("%s/members", basePath),
	}
}

func (c *membersClient) Get(ctx context.Context, params *service.GetMembersParams) (pagination.Pagination[service.Member], error) {
	var resp pagination.Pagination[service.Member]
	queryParameters := request.MarshallParams(params)
	if err := c.transport.GetJSON(ctx, c.path+queryParameters.Encode(), &resp); err != nil {
		return pagination.Pagination[service.Member]{}, err
	}

	return resp, nil
}

func (c *membersClient) GetID(ctx context.Context, id string) (service.Member, error) {
	var resp service.Member
	if err := c.transport.GetJSON(ctx, fmt.Sprintf("%s/%s", c.path, id), &resp); err != nil {
		return service.Member{}, err
	}

	return resp, nil
}
