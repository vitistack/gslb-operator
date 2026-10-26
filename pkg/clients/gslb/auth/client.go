package auth

import (
	"context"

	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport"
	authmodels "github.com/vitistack/gslb-operator/pkg/models/auth"
)

type Client interface {
	Token(context.Context, authmodels.LoginPayload) (authmodels.TokenResponse, error)
}

type AuthClient struct {
	transport transport.Transport
}

func NewAuthClient(transport transport.Transport) *AuthClient {
	return &AuthClient{
		transport: transport,
	}
}

func (c *AuthClient) Token(ctx context.Context, payload authmodels.LoginPayload) (authmodels.TokenResponse, error) {
	var resp authmodels.TokenResponse
	if err := c.transport.PostJSON(ctx, "/auth/token", payload, &resp); err != nil {
		return authmodels.TokenResponse{}, err
	}
	return resp, nil
}
