package checks

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

type HTTPChecker struct {
	*RoundTripper
	url       string
	client    *http.Client
	validator *LuaValidator
}

func NewHTTPChecker(url string, timeout time.Duration, validationScripts ...string) Checker {
	transport := http.DefaultTransport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	var validator *LuaValidator
	for _, script := range validationScripts {
		if script != "" {
			validator = &LuaValidator{script: script}
		}
	}

	return &HTTPChecker{
		RoundTripper: NewRoundtripper(),
		url:          url,
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		validator: validator,
	}
}

func (c *HTTPChecker) Check() *HealthCheckResult {
	c.startRecording()
	resp, err := c.client.Get(c.url)
	c.endRecording()
	defer resp.Body.Close()

	if err != nil {
		return &HealthCheckResult{
			err:       err,
			Success:   false,
			CheckTime: c.lastCheck(),
		}
	}

	if c.validator != nil { // run custom validation instead
		validateErr := c.validator.Validate(resp)
		return &HealthCheckResult{
			err:       validateErr,
			Success:   validateErr == nil,
			CheckTime: c.lastCheck(),
		}
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		return &HealthCheckResult{
			err:       fmt.Errorf("service un-available: %d", resp.StatusCode),
			Success:   false,
			CheckTime: c.lastCheck(),
		}
	}
	return &HealthCheckResult{
		err:       nil,
		Success:   true,
		CheckTime: c.lastCheck(),
	}
}

func (c *HTTPChecker) Roundtrip() time.Duration {
	return c.AverageRoundtripTime()
}
