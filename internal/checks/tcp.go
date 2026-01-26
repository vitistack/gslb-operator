package checks

import (
	"errors"
	"net"
	"time"

	tcpshaker "github.com/tevino/tcp-shaker"
)

type TCPChecker struct {
	addr    string
	timeout time.Duration
}

type TCPFullChecker struct {
	TCPChecker
}

func NewTCPChecker(typ, addr string, timeout time.Duration) Checker {
	switch typ {
	case TCP_FULL:
		return NewTCPFullChecker(addr, timeout)
	case TCP_HALF:
		return NewTCPHalfChecker(addr, timeout)
	}

	return nil
}

func NewTCPFullChecker(addr string, timeout time.Duration) Checker {
	return &TCPFullChecker{
		TCPChecker: TCPChecker{
			addr:    addr,
			timeout: timeout,
		},
	}
}

func (tf *TCPFullChecker) Check() error {
	conn, err := net.DialTimeout("tcp", tf.addr, tf.timeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

type TCPHalfChecker struct {
	TCPChecker
}

func NewTCPHalfChecker(addr string, timeout time.Duration) Checker {
	return &TCPHalfChecker{
		TCPChecker{
			addr:    addr,
			timeout: timeout,
		},
	}
}

func (th *TCPHalfChecker) Check() error {
	checker := tcpshaker.DefaultChecker()

	err := checker.CheckAddr(th.addr, th.timeout)
	if err != nil {
		if errors.Is(err, tcpshaker.ErrTimeout) {
			return err
		}
	}

	return nil
}
