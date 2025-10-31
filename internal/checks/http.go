package checks

import (
	"errors"
	"net/http"
	"time"
)

func HTTPCheck(url string, timeout time.Duration) func() error {
	return func() error {
		client := http.Client{
			Timeout: timeout,
		}
		resp, err := client.Get(url)
		if err != nil {
			if errors.Is(err, ErrServiceUnavailable) {
				return err
			}
		}
		resp.Body.Close()
		return nil
	}
}
