package rest

import (
	"net/http"
)

type Option func(*RestTransport)

func WithRoundTripper(trip http.RoundTripper) Option {
	return func(rt *RestTransport) {
		rt.httpClient.Transport = trip
	}
}
