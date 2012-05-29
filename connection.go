package nats

import (
	"bufio"
	"net"
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

func startReader(conn net.Conn) (<-chan readObject, <-chan error) {
	var brd = bufio.NewReader(conn)
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

	return robjch, rerrch
}

func startWriter(conn net.Conn) (chan<- writeObject, <-chan error) {
	var bwr = bufio.NewWriter(conn)
	var wobjch = make(chan writeObject)
	var werrch = make(chan error, 1)

	go func() {
		var err error

		defer close(werrch)

		for err == nil {
			select {
			case obj, ok := <-wobjch:
				if !ok {
					return
				}

				err = write(bwr, obj)
				if err != nil {
					werrch <- err
					break
				}

				err = bwr.Flush()
				if err != nil {
					werrch <- err
					break
				}
			}
		}

		// Drain writes channel, to make code that is blasting writes through this
		// channel doesn't block when an error occurs.
		for _ = range wobjch {
		}
	}()

	return wobjch, werrch
}

func (c *Connection) startPonger() chan<- bool {
	var pongch = make(chan bool)

	go func() {
		var stop bool
		var pongs int

		for !stop {
			if pongs == 0 {
				select {
				case _, ok := <-pongch:
					if !ok {
						stop = true
						break
					}

					pongs++
				}
			} else {
				select {
				case _, ok := <-pongch:
					if !ok {
						stop = true
						break
					}

					pongs++

				case wobjch, ok := <-c.wch:
					if !ok {
						stop = true
						break
					}

					for ; pongs > 0; pongs-- {
						wobjch <- &writePong{}
					}

					c.wch <- wobjch
				}
			}
		}
	}()

	return pongch
}

func (c *Connection) Run(conn net.Conn) error {
	var err error
	var stop bool

	pongch := c.startPonger()
	robjch, rerrch := startReader(conn)
	wobjch, werrch := startWriter(conn)

	c.wch <- wobjch

	for !stop {
		var robj readObject

		select {
		case <-c.stop:
			stop = true
		case err = <-rerrch:
			stop = true
		case err = <-werrch:
			stop = true
		case robj = <-robjch:
			switch robj.(type) {
			case *readPing:
				pongch <- true
			case *readPong:
				c.pingch <- true
			}
		}
	}

	conn.Close()

	// Stop ponger
	close(pongch)

	// Close reader
	for _ = range robjch {
	}
	for _ = range rerrch {
	}

	// Close writer
	close(<-c.wch)
	for _ = range werrch {
	}

	return err
}
