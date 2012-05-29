package nats

import (
	"io"
	"net"
	"testing"
	"bytes"
	"time"
)

type FakeAddr int

func (fa FakeAddr) Network() string {
	panic("FakeAddr#Network")
}

func (fa FakeAddr) String() string {
	panic("FakeAddr#String")
}

type FakeConnection struct {
	rin  *io.PipeReader
	win  *io.PipeWriter
	rout *io.PipeReader
	wout *io.PipeWriter
}

func NewFakeConnection() *FakeConnection {
	var fc FakeConnection

	fc.rin, fc.wout = io.Pipe()
	fc.rout, fc.win = io.Pipe()

	return &fc
}

func (fc *FakeConnection) Read(b []byte) (n int, err error) {
	return fc.rin.Read(b)
}

func (fc *FakeConnection) Write(b []byte) (n int, err error) {
	return fc.win.Write(b)
}

func (fc *FakeConnection) TestRead(t *testing.T, b string) bool {
	var err error

	var buf []byte
	var n int

	buf = make([]byte, len(b))
	n, err = fc.rout.Read(buf)

	if err != nil {
		t.Errorf("\nerror: %#v\n", err)
		return false
	}

	var expected []byte = []byte(b)
	var actual []byte = bytes.ToLower(buf[0:n])
	if !bytes.Equal(expected, actual) {
		t.Errorf("\nexpected: %#v\ngot: %#v\n", string(expected), string(actual))
		return false
	}

	return true
}

func (fc *FakeConnection) TestWrite(t *testing.T, b string) bool {
	var err error

	_, err = fc.wout.Write([]byte(b))

	if err != nil {
		t.Errorf("\nerror: %#v\n", err)
		return false
	}

	return true
}

func (fc *FakeConnection) Close() error {
	var err error

	err = fc.rin.Close()
	if err != nil {
		panic(err)
	}

	err = fc.win.Close()
	if err != nil {
		panic(err)
	}

	err = fc.rout.Close()
	if err != nil {
		panic(err)
	}

	err = fc.wout.Close()
	if err != nil {
		panic(err)
	}

	return nil
}

func (fc *FakeConnection) LocalAddr() net.Addr {
	var fa FakeAddr
	return fa
}

func (fc *FakeConnection) RemoteAddr() net.Addr {
	var fa FakeAddr
	return fa
}

func (fc *FakeConnection) SetDeadline(t time.Time) error {
	return nil
}

func (fc *FakeConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (fc *FakeConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

type testConnection struct {
	c    *Connection
	fc   *FakeConnection
	done chan bool
}

func (self *testConnection) start() {
	self.c = NewConnection()
	self.fc = NewFakeConnection()
	self.done = make(chan bool)

	go func() {
		self.c.Run(self.fc)
		self.done <- true
	}()
}

func (self *testConnection) stop() {
	// Close connection
	self.fc.Close()

	// Wait for goroutine
	<-self.done
}

func startTestConnection() *testConnection {
	var tc = new(testConnection)
	tc.start()
	return tc
}

func TestConnectionPongOnPing(t *testing.T) {
	var tc = startTestConnection()
	var ok bool

	// Write PING
	ok = tc.fc.TestWrite(t, "ping\r\n")
	if !ok {
		return
	}

	// Read PONG
	ok = tc.fc.TestRead(t, "pong\r\n")
	if !ok {
		return
	}

	tc.stop()
}
