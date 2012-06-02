package nats

import (
	"net"
	"time"
)

type Dialer interface {
	Dial() (net.Conn, error)
}

type DumbDialer struct {
	Conn net.Conn
}

func (d DumbDialer) Dial() (net.Conn, error) {
	return d.Conn, nil
}

type RetryingDialer struct {
	// The dialer
	f func(addr string) (net.Conn, error)

	// The sleeper
	s func(i uint)

	// Address to connect to
	Addr string

	// Maximum number of connection attempts
	MaxAttempts uint
}

func (d RetryingDialer) Dial() (net.Conn, error) {
	var i uint
	var n net.Conn
	var e error

	for ; ; i++ {
		if d.MaxAttempts > 0 && i >= d.MaxAttempts {
			break
		}

		n, e = d.f(d.Addr)
		if n != nil {
			return n, nil
		}

		d.s(i)
	}

	if e == nil {
		panic("expected an error")
	}

	return nil, e
}

func DefaultDialer(addr string) Dialer {
	var d RetryingDialer

	d.f = func(addr string) (net.Conn, error) {
		return net.Dial("tcp", addr)
	}

	d.s = func(i uint) {
		var exp uint = i + 3
		if exp > 12 {
			exp = 12
		}

		// Sleep between 8ms and 4096ms
		time.Sleep((1 << exp) * time.Millisecond)
	}

	d.Addr = addr

	// Retry forever
	d.MaxAttempts = 0

	return d
}
