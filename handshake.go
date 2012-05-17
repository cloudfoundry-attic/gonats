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
	var brd = bufio.NewReader(c)
	var bwr = bufio.NewWriter(c)
	var robj readObject
	var err error

	robj, err = read(brd)
	if err != nil {
		return nil, err
	}

	var rinfo *readInfo
	var ok bool

	rinfo, ok = robj.(*readInfo)
	if !ok {
		return nil, ErrExpectedInfo
	}

	if rinfo.SslRequired {
		c = tls.Client(c, &tls.Config{InsecureSkipVerify: true})
		brd = bufio.NewReader(c)
		bwr = bufio.NewWriter(c)
	}

	var wobj writeObject = &writeConnect{
		Verbose:  true,
		Pedantic: true,
		User:     user,
		Pass:     pass,
	}

	err = writeAndFlush(bwr, wobj)
	if err != nil {
		return nil, err
	}

	robj, err = read(brd)
	if err != nil {
		return nil, err
	}

	switch robj.(type) {
	case *readOk:
	case *readErr:
		err = ErrAuthenticationFailure
	default:
		panic("expected OK or ERR")
	}

	if err != nil {
		return nil, err
	}

	return c, nil
}

func handshake(c net.Conn, user, pass string) (net.Conn, error) {
	var cch = make(chan net.Conn, 1)
	var ech = make(chan error, 1)
	var err error

	go func() {
		c, err := _handshake(c, user, pass)

		if err != nil {
			ech <- err
			return
		}

		cch <- c
	}()

	select {
	case c = <-cch:
	case err = <-ech:
	case <-time.After(1 * time.Second):
		err = ErrHandshakeTimeout
	}

	if err != nil {
		return nil, err
	}

	return c, nil
}
