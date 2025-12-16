package client

import (
	"fmt"
	"net/http"
)

type optionContext struct {
	base    *Client
	wrapped HTTPClient
}

type clientOption func(ctx *optionContext) error

// WithRetry wraps the HTTPClient with a retry client to perform retry logic on request failure
func WithRetry(maxRetries int, retryOptions ...retryClientOption) clientOption {
	opts := make([]retryClientOption, 0, len(retryOptions))
	opts = append(opts, RetryClientWithMaxRetries(maxRetries))
	opts = append(opts, retryOptions...)

	return func(ctx *optionContext) error {
		retryClient, err := NewRetryClient(ctx.wrapped, opts...)
		if err != nil {
			return err
		}

		ctx.wrapped = retryClient
		return nil
	}
}

// WithAuthInterception adds a new http.RoundTripper to re-authenticate in case of a 401 - UnAuthorized
func WithAuthInterception(reAuth ReAuth) clientOption {
	return func(ctx *optionContext) error {
		base := ctx.base.Transport
		if base == nil {
			base = http.DefaultTransport
		}

		if reAuth == nil {
			return fmt.Errorf("re-authentication func cannot be nil")
		}

		ctx.base.Transport = NewAuthInterceptor(base, reAuth)
		return nil
	}
}

func WithRequestLogging(logger Logger) clientOption {
	return func(ctx *optionContext) error {
		base := ctx.base.Transport
		if base == nil {
			base = http.DefaultTransport
		}

		if logger == nil {
			return fmt.Errorf("cannot add request logging with a nil logger")
		}

		ctx.base.Transport = NewLogInterception(logger, base)
		return nil
	}
}

// RetryClientWithMaxRetries will add a maximum amount of retries if a request fails before an error is returned
func RetryClientWithMaxRetries(retries int) retryClientOption {
	return func(c *Retry) error {
		if retries < 0 {
			return fmt.Errorf("max retries cannot be negative")
		}
		c.MaxRetries = retries
		return nil
	}
}

// RetryClientWithRetryFunc sets the function that will be called to determine if a request should be retried
func RetryClientWithRetryFunc(f func(*http.Response, error) bool) retryClientOption {
	return func(c *Retry) error {
		if f == nil {
			return fmt.Errorf("retry function cannot be nil")
		}
		c.Retryable = f
		return nil
	}
}
