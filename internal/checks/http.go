package checks

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

func HTTPCheck(url string, timeout time.Duration) func() error {
	return func() error {
		customTransport := http.DefaultTransport.(*http.Transport).Clone()

		// Set InsecureSkipVerify to true in the TLS configuration
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		// Create a new HTTP client using the custom transport
		client := http.Client{
			Timeout:   timeout,
			Transport: customTransport,
		}
		resp, err := client.Get(url)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusServiceUnavailable { // service is physically up, but unable to respond
			return fmt.Errorf("server responded with statuscode: %d", resp.StatusCode)
		}
		resp.Body.Close()
		return nil
	}
}
