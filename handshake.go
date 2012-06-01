package nats

import (
	"errors"
	"net"
	"crypto/tls"
	"bufio"
	"time"
)

var (
	ErrExpectedInfo          = errors.New("nats: expected INFO")
	ErrAuthenticationFailure = errors.New("nats: authentication failed")
	ErrHandshakeTimeout      = errors.New("nats: handshake timed out")
)

type Handshaker interface {
	Handshake(net.Conn) (net.Conn, error)
	SetDeadline(time.Time) error
}

type ActualHandshaker struct {
	Username, Password string
	dt                 time.Duration
}

func (h *ActualHandshaker) handshake(c net.Conn) (net.Conn, error) {
	var r = bufio.NewReader(c)
	var w = bufio.NewWriter(c)
	var ro readObject
	var e error

	ro, e = read(r)
	if e != nil {
		return nil, e
	}

	var info *readInfo
	var ok bool

	info, ok = ro.(*readInfo)
	if !ok {
		return nil, ErrExpectedInfo
	}

	if info.SslRequired {
		c = tls.Client(c, &tls.Config{InsecureSkipVerify: true})
		r = bufio.NewReader(c)
		w = bufio.NewWriter(c)
	}

	var wo writeObject = &writeConnect{
		Verbose:  true,
		Pedantic: true,
		User:     h.Username,
		Pass:     h.Password,
	}

	e = writeAndFlush(w, wo)
	if e != nil {
		return nil, e
	}

	ro, e = read(r)
	if e != nil {
		return nil, e
	}

	switch ro.(type) {
	case *readOk:
	case *readErr:
		e = ErrAuthenticationFailure
	default:
		panic("expected OK or ERR")
	}

	if e != nil {
		return nil, e
	}

	return c, nil
}

func (h *ActualHandshaker) Handshake(c net.Conn) (net.Conn, error) {
	var cc = make(chan net.Conn, 1)
	var ec = make(chan error, 1)
	var e error

	go func() {
		c, e := h.handshake(c)

		if e != nil {
			ec <- e
			return
		}

		cc <- c
	}()

	var tc <-chan time.Time
	if h.dt != 0 {
		tc = time.After(h.dt)
	}

	select {
	case c = <-cc:
	case e = <-ec:
	case <-tc:
		e = ErrHandshakeTimeout
	}

	if e != nil {
		return nil, e
	}

	return c, nil
}

func (h *ActualHandshaker) SetTimeout(dt time.Duration) error {
	h.dt = dt
	return nil
}
