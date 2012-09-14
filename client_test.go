package nats

import (
	"nats/test"
	"net"
	"sync"
	"testing"
)

type testClient struct {
	*testing.T

	// Test client
	c *Client

	// Channel to receive the return value of cl()
	ec chan error

	// Channel to pass client side of the connection to Dialer
	ncc chan net.Conn

	// Test server
	s *test.TestServer

	// WaitGroup to join goroutines after every test
	sync.WaitGroup
}

func (tc *testClient) Setup(t *testing.T) {
	tc.T = t
	tc.c = NewClient()
	tc.ec = make(chan error, 1)
	tc.ncc = make(chan net.Conn)

	tc.Add(1)
	go func() {
		tc.ec <- tc.c.Run(DumbChannelDialer{tc.ncc}, EmptyHandshake)
		tc.Done()
	}()

	tc.ResetConnection()
}

func (tc *testClient) ResetConnection() {
	// Close current test server, if any
	if tc.s != nil {
		tc.s.Close()
	}

	nc, ns := net.Pipe()

	// Pass new client side of the connection to Dialer
	tc.ncc <- nc

	// New server
	tc.s = test.NewTestServer(tc.T, ns)
}

func (tc *testClient) Teardown() {
	tc.c.Stop()
	tc.Wait()
}

func TestClientCloseInboxOnStop(t *testing.T) {
	var tc testClient

	tc.Setup(t)

	tc.Add(1)
	go func() {
		sub := tc.c.NewSubscription("subject")
		sub.Subscribe()

		_, ok := <-sub.Inbox
		if ok {
			t.Errorf("Expected not OK")
		}

		tc.Done()
	}()

	tc.s.AssertRead("SUB subject 1\r\n")

	tc.Teardown()
}

func TestClientSubscriptionReceivesMessage(t *testing.T) {
	var tc testClient

	tc.Setup(t)

	tc.Add(1)
	go func() {
		sub := tc.c.NewSubscription("subject")
		sub.Subscribe()

		m := <-sub.Inbox

		expected := "payload"
		actual := string(m.Payload)

		if actual != expected {
			t.Errorf("Expected: %#v, got: %#v", expected, actual)
		}

		tc.Done()
	}()

	tc.s.AssertRead("SUB subject 1\r\n")
	tc.s.AssertWrite("MSG subject 1 7\r\npayload\r\n")

	tc.Teardown()
}

func TestClientSubscriptionUnsubscribe(t *testing.T) {
	var tc testClient

	tc.Setup(t)

	tc.Add(1)
	go func() {
		sub := tc.c.NewSubscription("subject")
		sub.Subscribe()
		sub.Unsubscribe()

		_, ok := <-sub.Inbox
		if ok {
			t.Errorf("Expected not OK")
		}

		tc.Done()
	}()

	tc.s.AssertRead("SUB subject 1\r\n")
	tc.s.AssertRead("UNSUB 1\r\n")

	tc.Teardown()
}

func TestClientSubscriptionWithQueue(t *testing.T) {
	var tc testClient

	tc.Setup(t)

	tc.Add(1)
	go func() {
		sub := tc.c.NewSubscription("subject")
		sub.SetQueue("queue")
		sub.Subscribe()

		tc.Done()
	}()

	tc.s.AssertRead("SUB subject queue 1\r\n")

	tc.Teardown()
}

func TestClientSubscriptionWithMaximum(t *testing.T) {
	var tc testClient

	tc.Setup(t)

	tc.Add(1)
	go func() {
		sub := tc.c.NewSubscription("subject")
		sub.SetMaximum(1)
		sub.Subscribe()

		var n = 0
		for _ = range sub.Inbox {
			n += 1
		}

		if n != 1 {
			t.Errorf("Expected to receive 1 message")
		}

		tc.Done()
	}()

	tc.s.AssertRead("SUB subject 1\r\n")
	tc.s.AssertRead("UNSUB 1 1\r\n")
	tc.s.AssertWrite("MSG subject 1 2\r\nhi\r\n")
	tc.s.AssertWrite("MSG subject 1 2\r\nhi\r\n")
	tc.s.AssertRead("UNSUB 1\r\n")

	tc.Teardown()
}

func TestClientSubscriptionReceivesMessageAfterReconnect(t *testing.T) {
	var tc testClient

	tc.Setup(t)

	tc.Add(1)
	go func() {
		sub := tc.c.NewSubscription("subject")
		sub.Subscribe()

		m := <-sub.Inbox

		expected := "payload"
		actual := string(m.Payload)

		if actual != expected {
			t.Errorf("Expected: %#v, got: %#v", expected, actual)
		}

		tc.Done()
	}()

	tc.s.AssertRead("SUB subject 1\r\n")

	tc.ResetConnection()

	tc.s.AssertRead("SUB subject 1\r\n")
	tc.s.AssertWrite("MSG subject 1 7\r\npayload\r\n")

	tc.Teardown()
}

func TestClientSubscriptionAdjustsMaximumAfterReconnect(t *testing.T) {
	var tc testClient

	tc.Setup(t)

	tc.Add(1)
	go func() {
		sub := tc.c.NewSubscription("subject")
		sub.SetMaximum(2)
		sub.Subscribe()

		var n = 0
		for _ = range sub.Inbox {
			n += 1
		}

		if n != 2 {
			t.Errorf("Expected to receive 2 message")
		}

		tc.Done()
	}()

	tc.s.AssertRead("SUB subject 1\r\n")
	tc.s.AssertRead("UNSUB 1 2\r\n")
	tc.s.AssertWrite("MSG subject 1 2\r\nhi\r\n")

	tc.ResetConnection()

	tc.s.AssertRead("SUB subject 1\r\n")
	tc.s.AssertRead("UNSUB 1 1\r\n")
	tc.s.AssertWrite("MSG subject 1 2\r\nhi\r\n")
	tc.s.AssertRead("UNSUB 1\r\n")

	tc.Teardown()
}

func TestClientPublish(t *testing.T) {
	var tc testClient

	tc.Setup(t)

	tc.Add(1)
	go func() {
		ok := tc.c.Publish("subject", []byte("message"))
		if !ok {
			t.Error("Expected success")
		}

		tc.Done()
	}()

	tc.s.AssertRead("PUB subject 7\r\nmessage\r\n")

	tc.Teardown()
}

func TestClientRequest(t *testing.T) {
	var tc testClient

	tc.Setup(t)

	tc.Add(1)
	go func() {
		ok := tc.c.Request("subject", []byte("message"), func(sub *Subscription) {
			for _ = range sub.Inbox {
				break
			}
		})

		if !ok {
			t.Error("Expected success")
		}

		tc.Done()
	}()

	tc.s.AssertMatch("SUB _INBOX\\.[0-9a-f]{26} 1\r\n")
	tc.s.AssertMatch("PUB subject _INBOX\\.[0-9a-f]{26} 7\r\nmessage\r\n")

	tc.Teardown()
}

func TestClientPublishAndConfirmSucceeds(t *testing.T) {
	var tc testClient

	tc.Setup(t)

	tc.Add(1)
	go func() {
		ok := tc.c.PublishAndConfirm("subject", []byte("message"))
		if !ok {
			t.Error("Expected success")
		}

		tc.Done()
	}()

	tc.s.AssertRead("PUB subject 7\r\nmessage\r\n")
	tc.s.AssertRead("PING\r\n")
	tc.s.AssertWrite("PONG\r\n")

	tc.Teardown()
}

func TestClientPublishAndConfirmFails(t *testing.T) {
	var tc testClient

	tc.Setup(t)

	tc.Add(1)
	go func() {
		ok := tc.c.PublishAndConfirm("subject", []byte("message"))
		if ok {
			t.Error("Expected failure")
		}

		tc.Done()
	}()

	tc.s.AssertRead("PUB subject 7\r\nmessage\r\n")
	tc.s.AssertRead("PING\r\n")

	tc.Teardown()
}
