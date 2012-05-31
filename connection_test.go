package nats

import (
	"net"
	"testing"
	"sync"
	"nats/test"
)

func testConnectionBootstrap(t *testing.T) (*Connection, *test.TestServer, *sync.WaitGroup) {
	var nc, ns = net.Pipe()
	var s = test.NewTestServer(t, ns)
	var c = NewConnection(nc)
	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		c.Run()
		wg.Done()
	}()

	return c, s, &wg
}

func TestConnectionPongOnPing(t *testing.T) {
	_, s, wg := testConnectionBootstrap(t)

	// Write PING
	s.AssertWrite("PING\r\n")

	// Read PONG
	s.AssertRead("PONG\r\n")

	s.Close()

	wg.Wait()
}

func TestConnectionPingWhenConnected(t *testing.T) {
	c, s, wg := testConnectionBootstrap(t)

	go func() {
		wg.Add(1)

		s.AssertRead("PING\r\n")
		s.AssertWrite("PONG\r\n")

		wg.Done()
	}()

	var ok bool = c.Ping()
	if !ok {
		t.Errorf("\nexpected ok\n")
	}

	s.Close()

	wg.Wait()
}

func TestConnectionPingWhenDisconnected(t *testing.T) {
	c, s, wg := testConnectionBootstrap(t)

	go func() {
		wg.Add(1)

		s.Close()

		wg.Done()
	}()

	var ok bool = c.Ping()
	if ok {
		t.Errorf("\nexpected not ok\n")
	}

	s.Close()

	wg.Wait()
}

func TestConnectionPingWhenDisconnectedMidway(t *testing.T) {
	c, s, wg := testConnectionBootstrap(t)

	go func() {
		wg.Add(1)

		s.AssertRead("PING\r\n")
		s.Close()

		wg.Done()
	}()

	var ok bool = c.Ping()
	if ok {
		t.Errorf("\nexpected not ok\n")
	}

	s.Close()

	wg.Wait()
}
