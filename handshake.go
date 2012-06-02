package nats

import (
	"errors"
	"net"
	"crypto/tls"
	"bufio"
)

var (
	ErrAuthenticationFailure = errors.New("nats: authentication failed")
	ErrHandshakeTimeout      = errors.New("nats: handshake timed out")
)

type Handshaker interface {
	Handshake(net.Conn) (net.Conn, error)
}

var EmptyHandshake = emptyHandshake{}

type emptyHandshake struct {
	// Not much...
}

func (h emptyHandshake) Handshake(n net.Conn) (net.Conn, error) {
	return n, nil
}

type Handshake struct {
	Username string
	Password string
}

func (h Handshake) Handshake(c net.Conn) (net.Conn, error) {
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
