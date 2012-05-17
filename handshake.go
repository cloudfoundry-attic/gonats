package nats

import (
	"errors"
	"net"
	"crypto/tls"
	"bufio"
)

var (
	ErrExpectedInfo         = errors.New("nats: expected INFO")
	ErrAuthenticationFailed = errors.New("nats: authentication failed")
)

func handshake(c net.Conn, user, pass string) (net.Conn, error) {
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
		err = ErrAuthenticationFailed
	default:
		panic("expected OK or ERR")
	}

	if err != nil {
		return nil, err
	}

	return c, nil
}
