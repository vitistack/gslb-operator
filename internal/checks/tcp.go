package checks

import (
	"net"
	"time"
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

func TCPHalf(addr string) func() error {
	return func() error {
		return nil
	}
}