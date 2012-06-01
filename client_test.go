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

		expected := "payload"
		actual := string(m.Payload)

		if actual != expected {
			t.Errorf("Expected: %#v, got: %#v", expected, actual)
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
			t.Errorf("Expected not OK")
		}

		wg.Done()
	}()

	s.AssertRead("SUB subject 1\r\n")
	s.AssertRead("UNSUB 1\r\n")
	s.Close()

	wg.Wait()
}

func TestClientSubscriptionWithQueue(t *testing.T) {
	c, s, wg := testClientBootstrap(t)

	go func() {
		wg.Add(1)

		sub := c.NewSubscription("subject")
		sub.SetQueue("queue")
		sub.Subscribe()

		wg.Done()
	}()

	s.AssertRead("SUB subject queue 1\r\n")
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
			t.Errorf("Expected to receive 1 message")
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

func TestClientPublish(t *testing.T) {
	c, s, wg := testClientBootstrap(t)

	go func() {
		wg.Add(1)

		ok := c.Publish("subject", []byte("message"))
		if !ok {
			t.Error("Expected success")
		}

		wg.Done()
	}()

	s.AssertRead("PUB subject 7\r\nmessage\r\n")
	s.Close()

	wg.Wait()
}

func TestClientPublishAndConfirmSucceeds(t *testing.T) {
	c, s, wg := testClientBootstrap(t)

	go func() {
		wg.Add(1)

		ok := c.PublishAndConfirm("subject", []byte("message"))
		if !ok {
			t.Error("Expected success")
		}

		wg.Done()
	}()

	s.AssertRead("PUB subject 7\r\nmessage\r\n")
	s.AssertRead("PING\r\n")
	s.AssertWrite("PONG\r\n")
	s.Close()

	wg.Wait()
}

func TestClientPublishAndConfirmFails(t *testing.T) {
	c, s, wg := testClientBootstrap(t)

	go func() {
		wg.Add(1)

		ok := c.PublishAndConfirm("subject", []byte("message"))
		if ok {
			t.Error("Expected failure")
		}

		wg.Done()
	}()

	s.AssertRead("PUB subject 7\r\nmessage\r\n")
	s.AssertRead("PING\r\n")
	s.Close()

	wg.Wait()
}
