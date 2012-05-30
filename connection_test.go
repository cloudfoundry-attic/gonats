package nats

import (
	"net"
	"testing"
	"bytes"
)

type testConnection struct {
	conn       *Connection
	serverConn net.Conn
	done       chan bool
}

func (self *testConnection) start() {
	var conn *Connection = NewConnection()
	var clientConn, serverConn net.Conn = net.Pipe()
	var done = make(chan bool)

	go func() {
		conn.Run(clientConn)
		done <- true
	}()

	self.conn = conn
	self.serverConn = serverConn
	self.done = done
}

func (self *testConnection) stop() {
	// Close connection
	self.Close()

	// Wait for goroutine
	<-self.done
}

func (self *testConnection) TestRead(t *testing.T, s string) bool {
	var buf []byte
	var n int
	var e error

	buf = make([]byte, len(s))
	if n, e = self.serverConn.Read(buf); e != nil {
		t.Errorf("\nerror: %#v\n", e)
		return false
	}

	var expected []byte = []byte(s)
	var actual []byte = bytes.ToLower(buf[0:n])

	if !bytes.Equal(expected, actual) {
		t.Errorf("\nexpected: %#v\ngot: %#v\n", string(expected), string(actual))
		return false
	}

	return true
}

func (self *testConnection) TestWrite(t *testing.T, s string) bool {
	var e error

	if _, e = self.serverConn.Write([]byte(s)); e != nil {
		t.Errorf("\nerror: %#v\n", e)
		return false
	}

	return true
}

func (self *testConnection) Close() {
	self.serverConn.Close()
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
	ok = tc.TestWrite(t, "ping\r\n")
	if !ok {
		return
	}

	// Read PONG
	ok = tc.TestRead(t, "pong\r\n")
	if !ok {
		return
	}

	tc.stop()
}

func TestConnectionPingWhenConnected(t *testing.T) {
	var tc = startTestConnection()

	go func() {
		tc.TestRead(t, "ping\r\n")
		tc.TestWrite(t, "pong\r\n")
	}()

	var ok bool = tc.conn.Ping()
	if !ok {
		t.Errorf("\nexpected ok\n")
	}

	tc.stop()
}

func TestConnectionPingWhenDisconnected(t *testing.T) {
	var tc = startTestConnection()

	go func() {
		tc.Close()
	}()

	var ok bool = tc.conn.Ping()
	if ok {
		t.Errorf("\nexpected not ok\n")
	}

	tc.stop()
}

func TestConnectionPingWhenDisconnectedMidway(t *testing.T) {
	var tc = startTestConnection()

	go func() {
		tc.TestRead(t, "ping\r\n")
		tc.Close()
	}()

	var ok bool = tc.conn.Ping()
	if ok {
		t.Errorf("\nexpected not ok\n")
	}

	tc.stop()
}
