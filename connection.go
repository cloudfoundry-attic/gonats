package nats

import (
	"bufio"
	"io"
	"net/textproto"
)

type Connection struct {
	stop chan bool
	wch  chan (chan<- writeObject)

	// Sequencer for PINGs/receiving corresponding PONGs
	pingseq textproto.Pipeline

	// Channel for receiving PONGs
	pingch chan bool
}

func NewConnection() *Connection {
	var c = new(Connection)

	c.stop = make(chan bool)
	c.wch = make(chan (chan<- writeObject), 1)

	c.pingch = make(chan bool)

	return c
}

// Take the write channel, send a PING and wait for a PONG.
func (c *Connection) pingAndWaitForPong(wobjch chan<- writeObject) bool {
	seq := c.pingseq.Next()
	wobjch <- &writePing{}
	c.wch <- wobjch

	// Wait in line for pong
	c.pingseq.StartResponse(seq)
	_, ok := <-c.pingch
	c.pingseq.EndResponse(seq)

	return ok
}

func (c *Connection) Ping() bool {
	wobjch, ok := <-c.wch
	if !ok {
		return false
	}

	return c.pingAndWaitForPong(wobjch)
}

type connectionReader struct {
	ch    <-chan readObject
	errch <-chan error
}

func (cr connectionReader) Stop() {
	// Drain channel to make goroutine return
	for _ = range cr.ch {
	}
}

func startReader(rw io.ReadWriter) connectionReader {
	var brd = bufio.NewReader(rw)
	var robjch = make(chan readObject)
	var rerrch = make(chan error, 1)

	go func() {
		var obj readObject
		var err error

		defer close(robjch)
		defer close(rerrch)

		for {
			obj, err = read(brd)
			if err != nil {
				rerrch <- err
				break
			}

			robjch <- obj
		}
	}()

	return connectionReader{ch: robjch, errch: rerrch}
}

type connectionWriter struct {
	ch    chan<- writeObject
	errch <-chan error
}

func (cw connectionWriter) Stop() {
	// Close channel to make goroutine return
	close(cw.ch)
}

func startWriter(rw io.ReadWriter) connectionWriter {
	var bwr = bufio.NewWriter(rw)
	var wobjch = make(chan writeObject)
	var werrch = make(chan error, 1)

	go func() {
		var obj writeObject
		var err error

		defer close(werrch)

		for obj = range wobjch {
			if err == nil {
				err = write(bwr, obj)
				if err != nil {
					werrch <- err
					continue
				}
				err = bwr.Flush()
				if err != nil {
					werrch <- err
					continue
				}
			}
		}
	}()

	return connectionWriter{ch: wobjch, errch: werrch}
}

type connectionPonger struct {
	ch chan<- bool
}

func (cp connectionPonger) Stop() {
	// Close channel to make goroutine return
	close(cp.ch)
}

func (c *Connection) startPonger() connectionPonger {
	var pongch = make(chan bool)

	go func() {
		for _ = range pongch {
			go func() {
				var wobjch chan<- writeObject
				var ok bool

				wobjch, ok = <-c.wch
				if ok {
					wobjch <- &writePong{}
					c.wch <- wobjch
				}
			}()
		}
	}()

	return connectionPonger{ch: pongch}
}

func (c *Connection) Run(rw io.ReadWriteCloser) error {
	var err error
	var stop bool

	cPonger := c.startPonger()
	cReader := startReader(rw)
	cWriter := startWriter(rw)

	c.wch <- cWriter.ch

	for !stop {
		var robj readObject

		select {
		case <-c.stop:
			stop = true
		case err = <-cReader.errch:
			stop = true
		case err = <-cWriter.errch:
			stop = true
		case robj = <-cReader.ch:
			switch robj.(type) {
			case *readPing:
				cPonger.ch <- true
			case *readPong:
				c.pingch <- true
			}
		}
	}

	// Re-acquire writer channel
	<-c.wch

	// Close connection
	rw.Close()

	// We can't receive any more PINGs
	close(c.pingch)

	cPonger.Stop()
	cReader.Stop()
	cWriter.Stop()

	return err
}
