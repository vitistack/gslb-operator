package client

import (
	"net/http"
)

type ReAuth func(*http.Request)

type AuthInterceptor struct {
	Transport http.RoundTripper
	ReAuth    ReAuth
}

func (a *AuthInterceptor) RoundTrip(req *http.Request) (*http.Response, error) {
	trip := a.Transport
	if trip == nil {
		trip = http.DefaultTransport
	}

	resp, err := trip.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		a.ReAuth(req)
		resp, err = trip.RoundTrip(req) // retry original request after re-authentication
	}

	return resp, err
}

func NewAuthInterceptor(baseTransport http.RoundTripper, reAuth ReAuth) http.RoundTripper {
	return &AuthInterceptor{
		Transport: baseTransport,
		ReAuth:    reAuth,
	}
}
