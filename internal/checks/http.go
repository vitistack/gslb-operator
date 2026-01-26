package checks

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

type HTTPChecker struct {
	url       string
	client    *http.Client
	validator *LuaValidator
}

func NewHTTPChecker(url string, timeout time.Duration, validationScripts ...string) *HTTPChecker {
	transport := http.DefaultTransport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	var validator *LuaValidator
	for _, script := range validationScripts {
		if script != "" {
			validator = &LuaValidator{script: script}
		}
	}

	return &HTTPChecker{
		url: url,
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		validator: validator,
	}
}

func (c *HTTPChecker) Check() error {
	resp, err := c.client.Get(c.url)
	if err != nil {
		return err
	}

	if c.validator != nil { // run custom validation instead
		return c.validator.Validate(resp)
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		return fmt.Errorf("service un-available: %d", resp.StatusCode)
	}

	resp.Body.Close()
	return nil
}
