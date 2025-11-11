package checks

import (
	"errors"
	"net"
	"time"

	tcpshaker "github.com/tevino/tcp-shaker"
)

func TCPFull(addr string, timeout time.Duration) func() error {
	return func() error {
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}
}

func TCPHalf(addr string, timeout time.Duration) func() error {
	return func() error {
		checker := tcpshaker.DefaultChecker()

		err := checker.CheckAddr(addr, timeout)
		if err != nil {
			if errors.Is(err, tcpshaker.ErrTimeout) {
				return err
			}
		}

		return nil
	}
}
