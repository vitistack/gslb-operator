package services

import (
	"context"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/services/members"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/services/override"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/services/spoofs"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/services/status"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
	"github.com/vitistack/gslb-operator/pkg/iter"
	"github.com/vitistack/gslb-operator/pkg/models/pagination"
	"github.com/vitistack/gslb-operator/pkg/rest/request"
)

type Client interface {
	Status() status.Client
	Spoofs() spoofs.Client
	Override() override.Client
	Members() members.Client
}

type servicesClient struct {
	transport transport.Transport
	path      string
}

func NewServiceClient(transport transport.Transport, memberOf ...string) *servicesClient {
	path := "/services"
	if len(memberOf) > 0 {
		path += "/" + memberOf[0]
	}
	return &servicesClient{
		transport: transport,
		path:      path,
	}
}

func (c *servicesClient) Get(ctx context.Context, params *pagination.PaginationParams) (pagination.Pagination[model.GSLBServiceGroup], error) {
	var resp = pagination.Pagination[model.GSLBServiceGroup]{}
	query := request.MarshallParams(params)
	if err := c.transport.GetJSON(ctx, c.path+query.Encode(), &resp); err != nil {
		return pagination.Pagination[model.GSLBServiceGroup]{}, err
	}
	return resp, nil
}

func (c *servicesClient) Follow(ctx context.Context) (iter.Iterator[pagination.Pagination[model.GSLBServiceGroup]], func() error) {
	var iterError error
	iterator := func(yield func(pagination.Pagination[model.GSLBServiceGroup]) bool) {
		var resp = pagination.Pagination[model.GSLBServiceGroup]{}
		err := c.transport.GetJSON(ctx, c.path, &resp)
		if err != nil {
			iterError = err
			return
		}

		if !yield(resp) {
			return
		}

		for resp.Next != nil {
			err := c.transport.GetJSON(ctx, *resp.Next, &resp)
			if err != nil {
				iterError = err
				return
			}

			if !yield(resp) {
				return
			}
		}
	}

	finish := func() error {
		return iterError
	}

	return iterator, finish
}

func (c *servicesClient) Status() status.Client {
	return status.NewStatusClient(c.transport, c.path)
}

func (c *servicesClient) Spoofs() spoofs.Client {
	return spoofs.NewSpoofClient(c.transport, c.path)
}

func (c *servicesClient) Override() override.Client {
	return override.NewOverrideClient(c.transport, c.path)
}

func (c *servicesClient) Members() members.Client {
	return members.NewMembersClient(c.transport, c.path)
}
