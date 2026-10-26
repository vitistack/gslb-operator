package m2m

import (
	"fmt"
	"net/http"

	"github.com/vitistack/gslb-operator/pkg/clients/gslb/auth"
)

// NewRoundTripper authenticates every request using creds, minting and
// refreshing M2M access tokens via authClient's token endpoint.
func NewRoundTripper(authClient auth.Client, creds Credentials) http.RoundTripper {
	return &authRoundTripper{tokens: newTokenSource(authClient, creds)}
}

type authRoundTripper struct {
	tokens *tokenSource
}

func (rt *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := rt.tokens.Token(req.Context())
	if err != nil {
		return nil, fmt.Errorf("gslb auth: %w", err)
	}

	req = req.Clone(req.Context())
	req.Header.Set("Authorization", token.String())

	return http.DefaultTransport.RoundTrip(req)
}
