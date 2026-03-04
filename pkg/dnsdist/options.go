package dnsdist

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type clientOption func(c *Client) error

func WithHost(host string) clientOption {
	return func(c *Client) error {
		ip := net.ParseIP(host)
		if ip == nil {
			return ErrCouldNotParseAddr
		}
		c.host = ip
		return nil
	}
}

func WithHostName(hostname string) clientOption {
	return func(c *Client) error {
		ips, err := net.LookupHost(hostname)
		if err != nil {
			return fmt.Errorf("DNS - lookup failed: %w", err)
		}

		c.host = net.IP(ips[0])
		return nil
	}
}

func WithPort(port string) clientOption {
	return func(c *Client) error {
		port = strings.TrimSpace(port)
		if port == "" {
			return ErrCouldNotParseAddr
		}
		// Ensure all characters are digits
		for _, r := range port {
			if r < '0' || r > '9' {
				return ErrCouldNotParseAddr
			}
		}

		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return ErrCouldNotParseAddr
		}

		c.port = port
		return nil
	}
}

func WithTimeout(timeout time.Duration) clientOption {
	return func(c *Client) error {
		c.timeout = timeout
		return nil
	}
}

func WithNumRetriesOnCommandFailure(retries int) clientOption {
	return func(c *Client) error {
		if retries < 0 {
			return ErrNegativeRetryCount
		}
		c.retries = retries
		return nil
	}
}
