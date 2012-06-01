package nats

import (
	"errors"
	"net"
	"crypto/tls"
	"bufio"
	"time"
)

var (
	ErrAuthenticationFailure = errors.New("nats: authentication failed")
	ErrHandshakeTimeout      = errors.New("nats: handshake timed out")
)

type Handshaker interface {
	Handshake(net.Conn) (net.Conn, error)
	SetUsername(string) error
	SetPassword(string) error
	SetTimeout(dt time.Duration) error
}

var NoHandshake = &noHandshake{}

type noHandshake struct {
	// Not much...
}

func (h *noHandshake) Handshake(n net.Conn) (net.Conn, error) {
	return n, nil
}

func (h *noHandshake) SetUsername(u string) error {
	return nil
}

func (h *noHandshake) SetPassword(p string) error {
	return nil
}

func (h *noHandshake) SetTimeout(dt time.Duration) error {
	return nil
}

type Handshake struct {
	Username string
	Password string
	dt       time.Duration
}

func (h *Handshake) handshake(c net.Conn) (net.Conn, error) {
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
		panic("expected INFO")
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

func (h *Handshake) Handshake(c net.Conn) (net.Conn, error) {
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
		c.Close()
		return nil, e
	}

	return c, nil
}

func (h *Handshake) SetUsername(u string) error {
	h.Username = u
	return nil
}

func (h *Handshake) SetPassword(p string) error {
	h.Password = p
	return nil
}

func (h *Handshake) SetTimeout(dt time.Duration) error {
	h.dt = dt
	return nil
}
