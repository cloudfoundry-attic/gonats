package nats

import (
	"testing"
	"os/exec"
	"time"
	"net"
	"bufio"
)

func startNATS(t *testing.T, arg ...string) *exec.Cmd {
	var cmd *exec.Cmd
	var err error

	cmd = exec.Command("nats-server", arg...)

	err = cmd.Start()
	if err != nil {
		t.Fatalf("error starting nats: %s\n", err)
	}

	return cmd
}

func connectNATS(t *testing.T, addr string) net.Conn {
	var conn net.Conn
	var err error
	var start = time.Now()

	for time.Since(start) < time.Second {
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		break
	}

	if err != nil {
		t.Fatalf("error connecting to nats: %s\n", err)
	}

	return conn
}

func testPing(t *testing.T, c net.Conn) {
	var brd = bufio.NewReader(c)
	var bwr = bufio.NewWriter(c)
	var wobj writeObject
	var robj readObject
	var err error

	wobj = &writePing{}

	err = writeAndFlush(bwr, wobj)
	if err != nil {
		t.Errorf("error writing: %s\n", err)
		return
	}

	robj, err = read(brd)
	if err != nil {
		t.Errorf("error reading: %s\n", err)
		return
	}

	_, ok := robj.(*readPong)
	if !ok {
		t.Errorf("expected: PONG, got: %#v\n", robj)
		return
	}

	return
}

func testHandshake(t *testing.T, user, pass string, ssl bool) {
	var args []string

	args = append(args, "--port", "8222")

	if len(user) > 0 && len(pass) > 0 {
		args = append(args, "--user", user)
		args = append(args, "--pass", pass)
	}

	if ssl {
		args = append(args, "--ssl")
	}

	var cmd = startNATS(t, args...)
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	var conn = connectNATS(t, "127.0.0.1:8222")
	var err error

	// Perform handshake
	conn, err = handshake(conn, user, pass)
	if err != nil {
		t.Errorf("error performing handshake: %s\n", err)
		return
	}

	testPing(t, conn)
}

func TestHandshakeWithoutAuth(t *testing.T) {
	testHandshake(t, "", "", false)
}

func TestHandshakeWithAuth(t *testing.T) {
	testHandshake(t, "john", "doe", false)
}

func TestHandshakeWithoutAuthWithSsl(t *testing.T) {
	testHandshake(t, "", "", true)
}

func TestHandshakeWithAuthWithSsl(t *testing.T) {
	testHandshake(t, "john", "doe", true)
}
