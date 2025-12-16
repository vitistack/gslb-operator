package client

import (
	"fmt"
	"net/http"
)

type retryClientOption func(c *Retry) error

type Retry struct {
	client     HTTPClient
	Retryable  func(*http.Response, error) bool
	MaxRetries int
}

func NewRetryClient(baseClient HTTPClient, opts ...retryClientOption) (HTTPClient, error) {
	client := &Retry{
		client:     baseClient,
		MaxRetries: 3,
		Retryable: func(resp *http.Response, err error) bool {
			return false
		},
	}

	for _, opt := range opts {
		if err := opt(client); err != nil {
			return nil, fmt.Errorf("could not create client: %s", err.Error())
		}
	}

	return client, nil
}

func (c *Retry) Do(req *http.Request) (*http.Response, error) {
	count := 0

	resp, err := c.client.Do(req)
	for c.Retryable(resp, err) && count < c.MaxRetries {
		resp, err = c.client.Do(req)
	}

	return resp, err
}
