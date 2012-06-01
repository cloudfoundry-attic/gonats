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

	sc chan bool

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

	c.sc = make(chan bool, 1)

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

func (c *Connection) releaseReader(r *bufio.Reader) {
	c.r = r
	c.rLock.Unlock()
}

func (c *Connection) acquireWriter() *bufio.Writer {
	c.wLock.Lock()
	return c.w
}

func (c *Connection) releaseWriter(w *bufio.Writer) {
	c.w = w
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
		c.releaseWriter(w)
		return false
	}

	seq := c.ps.Next()
	c.releaseWriter(w)

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

func (c *Connection) Write(o writeObject) bool {
	var w *bufio.Writer
	var e error

	w = c.acquireWriter()
	e = c.write(w, o)
	if e != nil {
		c.releaseWriter(w)
		return false
	}

	c.releaseWriter(w)
	return true
}

func (c *Connection) WriteAndPing(o writeObject) bool {
	var w *bufio.Writer
	var e error

	w = c.acquireWriter()
	e = c.write(w, o)
	if e != nil {
		c.releaseWriter(w)
		return false
	}

	return c.pingAndWaitForPong(w)
}

func (c *Connection) Stop() {
	c.sc <- true
}

func (c *Connection) Run() error {
	var r *bufio.Reader
	var rc chan readObject

	r = c.acquireReader()
	rc = make(chan readObject)

	defer c.releaseReader(r)

	go func() {
		var o readObject
		var e error

		defer close(rc)

		for {
			o, e = c.read(r)
			if e != nil {
				break
			}

			rc <- o
		}
	}()

	var stop bool
	var e error
	var ok bool

	for !stop {
		var o readObject

		select {
		case <-c.sc:
			stop = true
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
