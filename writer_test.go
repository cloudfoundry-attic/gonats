package nats

import (
	"bytes"
	"bufio"
	"testing"
)

func testWriteMatch(t *testing.T, obj writeObject, expected string) {
	var buf *bytes.Buffer = bytes.NewBuffer(nil)
	var wr *bufio.Writer = bufio.NewWriter(buf)
	var err error

	err = obj.write(wr)
	if err != nil {
		t.Errorf("\nerror: %#v\n", err)
		return
	}

	err = wr.Flush()
	if err != nil {
		t.Errorf("\nerror: %#v\n", err)
		return
	}

	var bExpected []byte = []byte(expected)
	var bActual []byte = buf.Bytes()

	if !bytes.Equal(bExpected, bActual) {
		t.Errorf("\nexpected: %#v\ngot: %#v\n", string(bExpected), string(bActual))
		return
	}
}

func TestWriteConnect(t *testing.T) {
	var obj = &writeConnect{
		Verbose:  true,
		Pedantic: true,
		User:     "joe",
		Pass:     "s3cr3t",
	}

	var expected = "CONNECT {\"verbose\":true,\"pedantic\":true,\"user\":\"joe\",\"pass\":\"s3cr3t\"}\r\n"

	testWriteMatch(t, obj, expected)
}

func TestWritePing(t *testing.T) {
	var obj = &writePing{}

	var expected = "PING\r\n"

	testWriteMatch(t, obj, expected)
}

func TestWritePong(t *testing.T) {
	var obj = &writePong{}

	var expected = "PONG\r\n"

	testWriteMatch(t, obj, expected)
}

func TestWriteSubscribe(t *testing.T) {
	var obj = &writeSubscribe{
		Sid:     1,
		Subject: "subject",
		Queue:   "queue",
	}

	var expected = "SUB subject queue 1\r\n"

	testWriteMatch(t, obj, expected)
}

func TestWriteSubscribeWithoutQueue(t *testing.T) {
	var obj = &writeSubscribe{
		Sid:     1,
		Subject: "subject",
	}

	var expected = "SUB subject 1\r\n"

	testWriteMatch(t, obj, expected)
}

func TestWriteUnsubscribe(t *testing.T) {
	var obj = &writeUnsubscribe{
		Sid:     1,
		Maximum: 5,
	}

	var expected = "UNSUB 1 5\r\n"

	testWriteMatch(t, obj, expected)
}

func TestWriteUnsubscribeWithoutMaximum(t *testing.T) {
	var obj = &writeUnsubscribe{
		Sid: 1,
	}

	var expected = "UNSUB 1\r\n"

	testWriteMatch(t, obj, expected)
}
