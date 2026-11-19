package status

import (
	"context"
	"fmt"

	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
	"github.com/vitistack/gslb-operator/pkg/iter"
	"github.com/vitistack/gslb-operator/pkg/models/pagination"
	serviceModels "github.com/vitistack/gslb-operator/pkg/models/service"
)

type Client interface {
	Get(context.Context) (serviceModels.GSLBServiceStatus, error)
}

type statusClient struct {
	transport transport.Transport
	path      string
}

func NewStatusClient(transport transport.Transport, basePath string) *statusClient {
	return &statusClient{
		transport: transport,
		path:      fmt.Sprintf("%s/status", basePath),
	}
}

func (c *statusClient) Get(ctx context.Context) (serviceModels.GSLBServiceStatus, error) {
	var status serviceModels.GSLBServiceStatus
	err := c.transport.GetJSON(ctx, c.path, &status)
	if err != nil {
		return serviceModels.GSLBServiceStatus{}, err
	}

	return status, nil
}

func (c *statusClient) List(ctx context.Context) (iter.Iterator[pagination.Pagination[serviceModels.GSLBServiceStatus]], func() error) {
	var iterError error
	iterator := func(yield func(pagination.Pagination[serviceModels.GSLBServiceStatus]) bool) {
		var resp = pagination.Pagination[serviceModels.GSLBServiceStatus]{}
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
