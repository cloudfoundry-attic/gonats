package nats

import (
	"net"
	"testing"
	"sync"
	"nats/test"
)

func testClientBootstrap(t *testing.T) (*Client, *test.TestServer, *sync.WaitGroup) {
	var nc, ns = net.Pipe()
	var s = test.NewTestServer(t, ns)
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
