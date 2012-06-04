package nats

import (
	"bufio"
	"io"
	"sync"
	"net/textproto"
)

type Connection struct {
	rw io.ReadWriteCloser

	r     *bufio.Reader
	w     *bufio.Writer
	re    error
	we    error
	rec   chan error
	wec   chan error
	rLock sync.Mutex
	wLock sync.Mutex

	// Stop channel channel, stop acknowledgement channel
	scc   chan chan bool
	sackc chan bool

	// Sequencer for PINGs/receiving corresponding PONGs
	ps textproto.Pipeline

	// Channel for receiving PONGs
	pc chan bool

	// Channel for receiving messages
	oc chan readObject
}

func NewConnection(rw io.ReadWriteCloser) *Connection {
	var c = new(Connection)

	c.rw = rw

	c.r = bufio.NewReader(rw)
	c.w = bufio.NewWriter(rw)
	c.rec = make(chan error, 1)
	c.wec = make(chan error, 1)

	c.scc = make(chan chan bool, 1)
	c.sackc = nil

	c.pc = make(chan bool)
	c.oc = make(chan readObject)

	return c
}

func (c *Connection) setReadError(e error) {
	if c.re == nil {
		c.re = e
		c.rec <- e
	}
}

func (c *Connection) setWriteError(e error) {
	if c.we == nil {
		c.we = e
		c.wec <- e
	}
}

func (c *Connection) acquireReader() *bufio.Reader {
	c.rLock.Lock()
	return c.r
}

func (c *Connection) releaseReader() {
	c.rLock.Unlock()
}

func (c *Connection) acquireWriter() *bufio.Writer {
	c.wLock.Lock()
	return c.w
}

func (c *Connection) releaseWriter() {
	c.wLock.Unlock()
}

func (c *Connection) read(r *bufio.Reader) (readObject, error) {
	var o readObject
	var e error

	o, e = read(r)
	if e != nil {
		c.setReadError(e)
		return nil, e
	}

	return o, nil
}

func (c *Connection) write(w *bufio.Writer, o writeObject) error {
	var e error

	e = write(w, o)
	if e != nil {
		c.setWriteError(e)
		return e
	}

	e = w.Flush()
	if e != nil {
		c.setWriteError(e)
		return e
	}

	return e
}

func (c *Connection) pingAndWaitForPong(w *bufio.Writer) bool {
	var e error

	// Write PING and grab sequence number
	e = c.write(w, &writePing{})
	if e != nil {
		c.releaseWriter()
		return false
	}

	seq := c.ps.Next()
	c.releaseWriter()

	// Wait for PONG
	c.ps.StartResponse(seq)
	_, ok := <-c.pc
	c.ps.EndResponse(seq)

	return ok
}

func (c *Connection) Ping() bool {
	var w *bufio.Writer

	w = c.acquireWriter()
	return c.pingAndWaitForPong(w)
}

func (c *Connection) WriteChannel(oc chan writeObject) bool {
	var w *bufio.Writer
	var e error

	w = c.acquireWriter()
	defer c.releaseWriter()

	// Write until EOF
	for o := range oc {
		if e == nil {
			e = c.write(w, o)
		}
	}

	return e == nil
}

func (c *Connection) Write(o writeObject) bool {
	var w *bufio.Writer
	var e error

	w = c.acquireWriter()
	e = c.write(w, o)
	if e != nil {
		c.releaseWriter()
		return false
	}

	c.releaseWriter()
	return true
}

func (c *Connection) WriteAndPing(o writeObject) bool {
	var w *bufio.Writer
	var e error

	w = c.acquireWriter()
	e = c.write(w, o)
	if e != nil {
		c.releaseWriter()
		return false
	}

	return c.pingAndWaitForPong(w)
}

func (c *Connection) Stop() {
	var sc chan bool

	select {
	case sc = <-c.scc:
	default:
	}

	if sc == nil {
		return
	}

	// Trigger stop
	close(sc)

	// Wait for acknowledgement
	<-c.sackc
}

func (c *Connection) Run() error {
	var r *bufio.Reader
	var rc chan readObject
	var sc chan bool

	r = c.acquireReader()
	defer c.releaseReader()
	rc = make(chan readObject)

	// Create stop acknowledgement channel
	// This doesn't need a lock because it can only be used after Stop() has
	// acquired the stop channel, which is not yet available at this point.
	c.sackc = make(chan bool)
	defer close(c.sackc)

	// Create stop channel
	sc = make(chan bool)
	c.scc <- sc

	go func() {
		var stop bool
		var o readObject
		var e error

		defer close(rc)

		for !stop {
			o, e = c.read(r)
			if e != nil {
				break
			}

			select {
			case rc <- o:
			case _, ok := <-sc:
				// Stop when stop channel is closed
				if !ok {
					stop = true
				}
			}
		}
	}()

	var stop bool
	var e error
	var ok bool

	for !stop {
		var o readObject

		select {
		case _, ok = <-sc:
			// Stop when stop channel is closed
			if !ok {
				stop = true
			}
		case e = <-c.rec:
			stop = true
		case e = <-c.wec:
			stop = true
		case o, ok = <-rc:
			if ok {
				switch o.(type) {
				case *readPing:
					go func() {
						c.Write(&writePong{})
					}()
				case *readPong:
					c.pc <- true
				default:
					c.oc <- o
				}
			}
		}
	}

	// Close connection
	c.rw.Close()

	// Drain readObject channel to make read goroutine quit
	for _ = range rc {
	}

	// Can't receive more PONGs
	close(c.pc)

	// Can't receive more messages
	close(c.oc)

	return e
}
