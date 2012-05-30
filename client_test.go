package nats

import (
	"net"
	"bytes"
	"testing"
	"sync"
)

type testServer struct {
	*testing.T
	n net.Conn
}

func NewTestServer(t *testing.T, n net.Conn) *testServer {
	var s = new(testServer)

	s.T = t
	s.n = n

	return s
}

func (s *testServer) AssertRead(v string) bool {
	var buf []byte
	var n int
	var e error

	buf = make([]byte, len(v))
	if n, e = s.n.Read(buf); e != nil {
		s.Errorf("\nerror: %#v\n", e)
		return false
	}

	var a []byte = []byte(v)
	var b []byte = buf[0:n]

	if !bytes.Equal(a, b) {
		s.Errorf("\nexpected: %#v\ngot: %#v\n", string(a), string(b))
		return false
	}

	return true
}

func (s *testServer) AssertWrite(v string) bool {
	var e error

	if _, e = s.n.Write([]byte(v)); e != nil {
		s.Errorf("\nerror: %#v\n", e)
		return false
	}

	return true
}

func (s *testServer) Close() {
	var e error

	if e = s.n.Close(); e != nil {
		panic(e)
	}
}

func testClientBootstrap(t *testing.T) (*Client, *testServer, *sync.WaitGroup) {
	var nc, ns = net.Pipe()
	var s = NewTestServer(t, ns)
	var c = NewClient("unused")
	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		c.runConnection(nc)
		wg.Done()
	}()

	return c, s, &wg
}

func TestClientSubscriptionReceivesMessage(t *testing.T) {
	c, s, wg := testClientBootstrap(t)

	go func() {
		wg.Add(1)

		sub := c.NewSubscription("subject")
		sub.Subscribe()

		m := <-sub.Inbox
		if string(m.Payload) != "payload" {
			t.Errorf("\nexpected \"payload\", got %#v\n", string(m.Payload))
		}

		wg.Done()
	}()

	s.AssertRead("SUB subject 1\r\n")
	s.AssertWrite("MSG subject 1 7\r\npayload\r\n")
	s.Close()

	wg.Wait()
}

func TestClientSubscriptionUnsubscribe(t *testing.T) {
	c, s, wg := testClientBootstrap(t)

	go func() {
		wg.Add(1)

		sub := c.NewSubscription("subject")
		sub.Subscribe()
		sub.Unsubscribe()

		_, ok := <-sub.Inbox
		if ok {
			t.Errorf("\nexpected inbox to be closed\n")
		}

		wg.Done()
	}()

	s.AssertRead("SUB subject 1\r\n")
	s.AssertRead("UNSUB 1\r\n")
	s.Close()

	wg.Wait()
}

func TestClientSubscriptionWithMaximum(t *testing.T) {
	c, s, wg := testClientBootstrap(t)

	go func() {
		wg.Add(1)

		sub := c.NewSubscription("subject")
		sub.SetMaximum(1)
		sub.Subscribe()

		var n = 0
		for _ = range sub.Inbox {
			n += 1
		}

		if n != 1 {
			t.Errorf("\nexpected to receive 1 message\n")
		}

		wg.Done()
	}()

	s.AssertRead("SUB subject 1\r\n")
	s.AssertRead("UNSUB 1 1\r\n")
	s.AssertWrite("MSG subject 1 2\r\nhi\r\n")
	s.AssertWrite("MSG subject 1 2\r\nhi\r\n")
	s.Close()

	wg.Wait()
}
