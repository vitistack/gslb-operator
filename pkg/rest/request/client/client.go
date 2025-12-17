package client

import (
	"fmt"
	"net/http"
	"time"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	http.Client
}

func NewClient(timeout time.Duration, opts ...clientOption) (*HTTPClient, error) {
	baseClient := &Client{
		http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}

	ctx := &optionContext{
		base:    baseClient,
		wrapped: baseClient,
	}

	for _, opt := range opts {
		err := opt(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not create client: %s", err.Error())
		}
	}

	return &ctx.wrapped, nil
}


func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.Client.Do(req)
}
