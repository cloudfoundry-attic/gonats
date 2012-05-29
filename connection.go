package nats

import (
	"bufio"
	"io"
	"net/textproto"
)

type Connection struct {
	stop    chan bool
	writeCh chan (chan<- writeObject)

	// Sequencer for PINGs/receiving corresponding PONGs
	pingSeq textproto.Pipeline

	// Channel for receiving PONGs
	pingCh chan bool
}

func NewConnection() *Connection {
	var c = new(Connection)

	c.stop = make(chan bool)
	c.writeCh = make(chan (chan<- writeObject), 1)

	c.pingCh = make(chan bool)

	return c
}

// Take the write channel, send a PING and wait for a PONG.
func (c *Connection) pingAndWaitForPong(wc chan<- writeObject) bool {
	seq := c.pingSeq.Next()
	wc <- &writePing{}
	c.writeCh <- wc

	// Wait in line for pong
	c.pingSeq.StartResponse(seq)
	_, ok := <-c.pingCh
	c.pingSeq.EndResponse(seq)

	return ok
}

func (c *Connection) Ping() bool {
	wc, ok := <-c.writeCh
	if !ok {
		return false
	}

	return c.pingAndWaitForPong(wc)
}

type connectionReader struct {
	ch    <-chan readObject
	errCh <-chan error
}

func (cr connectionReader) Stop() {
	// Drain channel to make goroutine return
	for _ = range cr.ch {
	}
}

func startReader(rw io.ReadWriter) connectionReader {
	var brd = bufio.NewReader(rw)
	var oc = make(chan readObject)
	var ec = make(chan error, 1)

	go func() {
		var o readObject
		var e error

		defer close(oc)
		defer close(ec)

		for {
			o, e = read(brd)
			if e != nil {
				ec <- e
				break
			}

			oc <- o
		}
	}()

	return connectionReader{ch: oc, errCh: ec}
}

type connectionWriter struct {
	ch    chan<- writeObject
	errCh <-chan error
}

func (cw connectionWriter) Stop() {
	// Close channel to make goroutine return
	close(cw.ch)
}

func startWriter(rw io.ReadWriter) connectionWriter {
	var bwr = bufio.NewWriter(rw)
	var oc = make(chan writeObject)
	var ec = make(chan error, 1)

	go func() {
		var o writeObject
		var e error

		defer close(ec)

		for o = range oc {
			if e == nil {
				e = write(bwr, o)
				if e != nil {
					ec <- e
					continue
				}
				e = bwr.Flush()
				if e != nil {
					ec <- e
					continue
				}
			}
		}
	}()

	return connectionWriter{ch: oc, errCh: ec}
}

type connectionPonger struct {
	ch chan<- bool
}

func (cp connectionPonger) Stop() {
	// Close channel to make goroutine return
	close(cp.ch)
}

func (c *Connection) startPonger() connectionPonger {
	var pc = make(chan bool)

	go func() {
		for _ = range pc {
			go func() {
				var wc chan<- writeObject
				var ok bool

				wc, ok = <-c.writeCh
				if ok {
					wc <- &writePong{}
					c.writeCh <- wc
				}
			}()
		}
	}()

	return connectionPonger{ch: pc}
}

func (c *Connection) Run(rw io.ReadWriteCloser) error {
	var err error
	var stop bool

	cPonger := c.startPonger()
	cReader := startReader(rw)
	cWriter := startWriter(rw)

	c.writeCh <- cWriter.ch

	for !stop {
		var robj readObject

		select {
		case <-c.stop:
			stop = true
		case err = <-cReader.errCh:
			stop = true
		case err = <-cWriter.errCh:
			stop = true
		case robj = <-cReader.ch:
			switch robj.(type) {
			case *readPing:
				cPonger.ch <- true
			case *readPong:
				c.pingCh <- true
			}
		}
	}

	// Re-acquire writer channel
	<-c.writeCh

	// Close connection
	rw.Close()

	// We can't receive any more PINGs
	close(c.pingCh)

	cPonger.Stop()
	cReader.Stop()
	cWriter.Stop()

	return err
}
