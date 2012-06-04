package nats

import (
	"net"
	"io"
	"testing"
	"sync"
	"nats/test"
)

type testConnection struct {
	// Network pipe
	nc, ns net.Conn

	// Test server
	s *test.TestServer

	// Test connection
	c *Connection

	// Channel to receive the return value of c.Run()
	ec chan error

	// WaitGroup to join goroutines after every test
	sync.WaitGroup
}

func (tc *testConnection) Setup(t *testing.T) {
	tc.nc, tc.ns = net.Pipe()
	tc.s = test.NewTestServer(t, tc.ns)
	tc.c = NewConnection(tc.nc)
	tc.ec = make(chan error, 1)

	tc.Add(1)
	go func() {
		tc.ec <- tc.c.Run()
		tc.Done()
	}()
}

func (tc *testConnection) Teardown() {
	tc.c.Stop()
	tc.Wait()
}

func TestConnectionReturnNilOnStop(t *testing.T) {
	var tc testConnection

	tc.Setup(t)

	// Stop from goroutine
	tc.Add(1)
	go func() {
		tc.c.Stop()
		tc.Done()
	}()

	e, ok := <-tc.ec
	if !ok {
		t.Errorf("Expected OK")
	}

	if e != nil {
		t.Error(e)
	}

	tc.Teardown()
}

func TestConnectionReturnErrorOnError(t *testing.T) {
	var tc testConnection

	tc.Setup(t)

	// Stop from goroutine
	tc.Add(1)
	go func() {
		tc.s.Close()
		tc.Done()
	}()

	e, _ := <-tc.ec
	if e != io.EOF {
		t.Errorf("Expected: %#v, got: %#v", io.EOF, e)
	}

	tc.Teardown()
}

func TestConnectionPongOnPing(t *testing.T) {
	var tc testConnection

	tc.Setup(t)

	// Write PING
	tc.s.AssertWrite("PING\r\n")

	// Read PONG
	tc.s.AssertRead("PONG\r\n")

	tc.Teardown()
}

func TestConnectionPingWhenConnected(t *testing.T) {
	var tc testConnection

	tc.Setup(t)

	tc.Add(1)
	go func() {
		tc.s.AssertRead("PING\r\n")
		tc.s.AssertWrite("PONG\r\n")
		tc.Done()
	}()

	var ok bool = tc.c.Ping()
	if !ok {
		t.Errorf("Expected OK")
	}

	tc.Teardown()
}

func TestConnectionPingWhenDisconnected(t *testing.T) {
	var tc testConnection

	tc.Setup(t)

	tc.Add(1)
	go func() {
		tc.s.Close()
		tc.Done()
	}()

	var ok bool = tc.c.Ping()
	if ok {
		t.Errorf("Expected not OK")
	}

	tc.Teardown()
}

func TestConnectionPingWhenDisconnectedMidway(t *testing.T) {
	var tc testConnection

	tc.Setup(t)

	tc.Add(1)
	go func() {
		tc.s.AssertRead("PING\r\n")
		tc.s.Close()
		tc.Done()
	}()

	var ok bool = tc.c.Ping()
	if ok {
		t.Errorf("Expected not OK")
	}

	tc.Teardown()
}
