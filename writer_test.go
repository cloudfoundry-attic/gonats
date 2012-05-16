package protocol

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
		User: "joe",
		Pass: "s3cr3t",
	}

	var expected = "CONNECT {\"user\":\"joe\",\"pass\":\"s3cr3t\"}\r\n"

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
