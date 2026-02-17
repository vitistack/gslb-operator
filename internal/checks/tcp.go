package checks

import (
	"errors"
	"net"
	"time"

	tcpshaker "github.com/tevino/tcp-shaker"
)

type TCPChecker struct {
	*RoundTripper
	addr    string
	timeout time.Duration
}

func (c *TCPChecker) Roundtrip() time.Duration {
	return c.AverageRoundtripTime()
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
		TCPChecker{
			RoundTripper: NewRoundtripper(),
			addr:         addr,
			timeout:      timeout,
		},
	}
}

func (tf *TCPFullChecker) Check() error {
	tf.startRecording()
	conn, err := net.DialTimeout("tcp", tf.addr, tf.timeout)
	tf.endRecording()
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
			RoundTripper: NewRoundtripper(),
			addr:         addr,
			timeout:      timeout,
		},
	}
}

func (th *TCPHalfChecker) Check() error {
	checker := tcpshaker.DefaultChecker()

	th.startRecording()
	err := checker.CheckAddr(th.addr, th.timeout)
	th.endRecording()
	if err != nil {
		if errors.Is(err, tcpshaker.ErrTimeout) {
			return err
		}
	}

	return nil
}
