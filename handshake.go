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

func _handshake(c net.Conn, user, pass string) (net.Conn, error) {
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
		User:     user,
		Pass:     pass,
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

func handshake(c net.Conn, user, pass string) (net.Conn, error) {
	var cc = make(chan net.Conn, 1)
	var ec = make(chan error, 1)
	var e error

	go func() {
		c, e := _handshake(c, user, pass)

		if e != nil {
			ec <- e
			return
		}

		cc <- c
	}()

	select {
	case c = <-cc:
	case e = <-ec:
	case <-time.After(1 * time.Second):
		e = ErrHandshakeTimeout
	}

	if e != nil {
		return nil, e
	}

	return c, nil
}
