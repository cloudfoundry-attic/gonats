package nats

import (
	"net"
	"testing"
	"fmt"
	"nats/test"
	"sync"
	"time"
)

func testHandshake(t *testing.T, user, pass string, ssl bool) {
	c, s := net.Pipe()
	srv := test.NewTestServer(t, s)
	wg := sync.WaitGroup{}

	wg.Add(1)

	go func() {
		h := ActualHandshaker{
			Username: user,
			Password: pass,
		}

		_, e := h.Handshake(c)
		if e != nil {
			t.Error(e)
		}

		wg.Done()
	}()

	var p string

	if ssl {
		p = "true"
	} else {
		p = "false"
	}

	p = fmt.Sprintf("INFO {\"ssl_required\":%s}\r\n", p)
	srv.AssertWrite(p)

	if ssl {
		srv.StartTLS()
	}

	p = fmt.Sprintf("CONNECT {\"verbose\":true,\"pedantic\":true,\"user\":\"%s\",\"pass\":\"%s\"}\r\n", user, pass)
	srv.AssertRead(p)

	p = fmt.Sprintf("+OK\r\n")
	srv.AssertWrite(p)

	wg.Wait()
}

func TestHandshakeWithoutAuth(t *testing.T) {
	testHandshake(t, "", "", false)
}

func TestHandshakeWithAuth(t *testing.T) {
	testHandshake(t, "john", "doe", false)
}

func TestHandshakeWithoutAuthWithSsl(t *testing.T) {
	testHandshake(t, "", "", true)
}

func TestHandshakeWithAuthWithSsl(t *testing.T) {
	testHandshake(t, "john", "doe", true)
}

func TestHandshakeTimeout(t *testing.T) {
	c, s := net.Pipe()
	srv := test.NewTestServer(t, s)
	wg := sync.WaitGroup{}

	wg.Add(1)

	go func() {
		h := ActualHandshaker{}
		h.SetTimeout(1 * time.Millisecond)

		_, e := h.Handshake(c)
		if e != ErrHandshakeTimeout {
			t.Error(e)
		}

		wg.Done()
	}()

	wg.Wait()
}
