package m2m

import (
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/auth"
	"github.com/vitistack/gslb-operator/pkg/clients/gslb/transport/rest"
)

func WithCredentials(creds Credentials) rest.Option {
	return func(rt *rest.RestTransport) {
		authClient := auth.NewAuthClient(rt)
		rest.WithRoundTripper(NewRoundTripper(authClient, creds))(rt)
	}
}
