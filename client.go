package nats

import (
	"net"
	"sync"
)

type Subscription struct {
	sr *subscriptionRegistry

	// Reference the connection the subscription was subscribed on
	c *Connection

	sid      uint
	frozen   bool
	maximum  uint
	received uint

	subject string
	queue   string

	Inbox chan *readMessage
}

func (s *Subscription) freeze() {
	if s.frozen {
		panic("subscription is frozen")
	}

	s.frozen = true
}

func (s *Subscription) SetSubject(v string) {
	if s.frozen {
		panic("subscription is frozen")
	}

	s.subject = v
}

func (s *Subscription) SetQueue(v string) {
	if s.frozen {
		panic("subscription is frozen")
	}

	s.queue = v
}

func (s *Subscription) SetMaximum(v uint) {
	if s.frozen {
		panic("subscription is frozen")
	}

	s.maximum = v
}

// Proxy to registry
func (s *Subscription) Subscribe() {
	s.sr.Subscribe(s)
}

// Expects to be called when the registry lock is held
func (s *Subscription) subscribe(c *Connection) {
	if s.c == c {
		// Already subscribed on this connection, nothing left to do
		return
	}

	// Reference the connection the subscription was subscribed on
	s.c = c

	// Use channel to pass writes to connection to retain atomicity
	wc := make(chan writeObject, 2)

	// Write subscribe command
	ws := new(writeSubscribe)
	ws.Sid = s.sid
	ws.Subject = s.subject
	ws.Queue = s.queue
	wc <- ws

	// Write automatic unsubscribe command if maximum is set
	if s.maximum > 0 {
		wu := new(writeUnsubscribe)
		wu.Sid = s.sid
		wu.Maximum = s.maximum
		wc <- wu
	}

	close(wc)

	// Pass writes to connection
	c.WriteChannel(wc)
}

// Proxy to registry
func (s *Subscription) Unsubscribe() {
	s.sr.Unsubscribe(s)
}

// Expects to be called when the registry lock is held
func (s *Subscription) unsubscribe() {
	// Since this subscription is now removed from the registry, it will no
	// longer receive messages and the inbox can be closed
	close(s.Inbox)

	wu := new(writeUnsubscribe)
	wu.Sid = s.sid
	s.c.Write(wu)
}

func (s *Subscription) deliver(m *readMessage) {
	s.received++
	s.Inbox <- m

	// Unsubscribe if the maximum number of messages has been received
	if s.maximum > 0 && s.received >= s.maximum {
		s.Unsubscribe()
	}
}

type subscriptionRegistry struct {
	sync.Mutex
	*Client

	sid uint
	m   map[uint]*Subscription
}

func (sr *subscriptionRegistry) emptyMap() {
	sr.m = make(map[uint]*Subscription)
}

func (sr *subscriptionRegistry) setup(c *Client) {
	sr.Client = c

	sr.emptyMap()
}

func (sr *subscriptionRegistry) teardown() {
	sr.Lock()
	defer sr.Unlock()

	for _, s := range sr.m {
		close(s.Inbox)
	}

	sr.emptyMap()
}

func (sr *subscriptionRegistry) NewSubscription(sub string) *Subscription {
	var s = new(Subscription)

	sr.Lock()

	s.sr = sr

	sr.sid++
	s.sid = sr.sid

	sr.Unlock()

	s.SetSubject(sub)
	s.Inbox = make(chan *readMessage)

	return s
}

func (sr *subscriptionRegistry) Subscribe(s *Subscription) {
	var c = sr.Client.AcquireConnection()

	sr.Lock()
	defer sr.Unlock()

	sr.m[s.sid] = s
	s.freeze()
	s.subscribe(c)
}

func (sr *subscriptionRegistry) Unsubscribe(s *Subscription) {
	sr.Lock()
	defer sr.Unlock()

	delete(sr.m, s.sid)
	s.unsubscribe()
}

func (sr *subscriptionRegistry) Deliver(m *readMessage) {
	var s *Subscription
	var ok bool

	sr.Lock()
	s, ok = sr.m[m.SubscriptionId]
	sr.Unlock()

	if ok {
		s.deliver(m)
	}
}

type Client struct {
	subscriptionRegistry
	Stopper

	cc chan *Connection
}

func NewClient() *Client {
	var t = new(Client)

	t.subscriptionRegistry.setup(t)

	t.cc = make(chan *Connection)

	return t
}

func (t *Client) AcquireConnection() *Connection {
	var c *Connection
	var ok bool

	c, ok = <-t.cc
	if !ok {
		return nil
	}

	return c
}

func (t *Client) Write(o writeObject) bool {
	c := t.AcquireConnection()
	if c == nil {
		return false
	}

	return c.Write(o)
}

func (t *Client) Ping() bool {
	c := t.AcquireConnection()
	if c == nil {
		return false
	}

	return c.Ping()
}

func (t *Client) publish(s string, m []byte, confirm bool) bool {
	var o = new(writePublish)

	o.Subject = s
	o.Message = m

	c := t.AcquireConnection()
	if c == nil {
		return false
	}

	ok := c.Write(o)
	if !ok {
		return false
	}

	// Round trip to confirm the publish was received
	if confirm {
		return c.Ping()
	}

	return true
}

func (t *Client) Publish(s string, m []byte) bool {
	return t.publish(s, m, false)
}

func (t *Client) PublishAndConfirm(s string, m []byte) bool {
	return t.publish(s, m, true)
}

func (t *Client) runConnection(n net.Conn, sc chan bool) error {
	var e error
	var c *Connection
	var dc chan bool

	c = NewConnection(n)
	dc = make(chan bool)

	// Feed connection until stop
	go func() {
		for {
			select {
			case t.cc <- c:
			case <-sc:
				c.Stop()
				return
			case <-dc:
				return
			}
		}
	}()

	// Read messages until EOF
	go func() {
		var o readObject

		for o = range c.oc {
			switch oo := o.(type) {
			case *readMessage:
				t.Deliver(oo)
			}
		}
	}()

	e = c.Run()
	close(dc)

	return e
}

func (t *Client) Run(d Dialer, h Handshaker) error {
	// There will not be more connections after Run returns
	defer close(t.cc)

	// There will not be more messages after Run returns
	defer t.subscriptionRegistry.teardown()

	var sc = t.MarkStart()
	defer t.MarkStop()

	var n net.Conn
	var e error

	for {
		n, e = d.Dial()
		if e != nil {
			// Error: dialer couldn't establish a connection
			return e
		}

		n, e = h.Handshake(n)
		if e != nil {
			// Error: handshake couldn't complete
			return e
		}

		e = t.runConnection(n, sc)
		if e == nil {
			// No error: client was explicitly stopped
			return nil
		}
	}

	return nil
}

func (t *Client) RunWithDefaults(addr string, user, pass string) error {
	d := DefaultDialer(addr)
	h := DefaultHandshaker(user, pass)
	return t.Run(d, h)
}
