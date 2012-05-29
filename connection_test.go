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

func (fc *FakeConnection) Close() error {
	err1 := fc.rin.Close()
	err2 := fc.win.Close()

	if err1 != nil {
		panic(err1)
	}

	if err2 != nil {
		panic(err2)
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

func writePingReadPong(t *testing.T, fc *FakeConnection) {
	var err error

	// Write PING
	_, err = fc.wout.Write([]byte("ping\r\n"))

	if err != nil {
		t.Errorf("\nerror: %#v\n", err)
		return
	}

	// Read PONG
	var buf []byte
	var n int

	buf = make([]byte, 16)
	n, err = fc.rout.Read(buf)

	if err != nil {
		t.Errorf("\nerror: %#v\n", err)
		return
	}

	var expected []byte = []byte("pong\r\n")
	var actual []byte = bytes.ToLower(buf[0:n])
	if !bytes.Equal(expected, actual) {
		t.Errorf("\nexpected: %#v\ngot: %#v\n", string(expected), string(actual))
		return
	}
}

func TestConnectionPongOnPing(t *testing.T) {
	var c = NewConnection()
	var fc = NewFakeConnection()
	var done = make(chan bool)

	go func() {
		c.Run(fc)
		done <- true
	}()

	writePingReadPong(t, fc)

	// Close connection
	fc.Close()

	// Wait for goroutine
	<-done
}
