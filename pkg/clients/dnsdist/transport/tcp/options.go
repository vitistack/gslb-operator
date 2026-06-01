package tcp

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

type tcpTransportOption func(*tcpTransport) error

func WithHost(host string) tcpTransportOption {
	return func(t *tcpTransport) error {
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("invalid host option: failed to parse ip: %s", host)
		}
		t.hostIP = ip
		return nil
	}
}

func WithHostName(hostname string) tcpTransportOption {
	return func(t *tcpTransport) error {
		ips, err := net.LookupHost(hostname)
		if err != nil {
			return fmt.Errorf("DNS - lookup failed for hostnam: %s: %w", hostname, err)
		}

		t.hostIP = net.IP(ips[0])
		return nil
	}
}

func WithPort(port uint16) tcpTransportOption {
	return func(t *tcpTransport) error {
		t.hostPort = strconv.Itoa(int(port))
		return nil
	}
}

func WithTimeout(timeout time.Duration) tcpTransportOption {
	return func(t *tcpTransport) error {
		t.timeout = timeout
		return nil
	}
}

func WithNumRetriesOnCommandFailure(retries int) tcpTransportOption {
	return func(t *tcpTransport) error {
		if retries < 0 {
			return fmt.Errorf("invalid retries options: retries not greater than or equal to 0")
		}
		t.retries = retries
		return nil
	}
}
