package nats

import (
	"net"
	"testing"
	"sync"
	"nats/test"
)

type testClient struct {
	// Network pipe
	nc, ns net.Conn

	// Test server
	s *test.TestServer

	// Test client
	c *Client

	// Channel to receive the return value of cl()
	ec chan error

	// WaitGroup to join goroutines after every test
	sync.WaitGroup
}

func (tc *testClient) Setup(t *testing.T) {
	tc.nc, tc.ns = net.Pipe()
	tc.s = test.NewTestServer(t, tc.ns)
	tc.c = NewClient("unused")
	tc.ec = make(chan error, 1)

	tc.Add(1)
	go func() {
		tc.ec <- tc.c.Run(tc.nc, NoHandshake)
		tc.Done()
	}()
}

func (tc *testClient) Teardown() {
	// Close test server
	tc.s.Close()

	// Wait for goroutines
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

	// Stop client
	tc.c.Stop()

	// Wait before closing server connection to avoid a race with EOF
	tc.Wait()

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
