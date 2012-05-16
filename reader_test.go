package nats

import (
	"reflect"
	"bufio"
	"strings"
	"testing"
)

// Create a bufio.Reader
func createReader(payload string) *bufio.Reader {
	sio := strings.NewReader(payload)
	return bufio.NewReader(sio)
}

func testReadMatch(t *testing.T, payload string, expected readObject) {
	var rd *bufio.Reader
	rd = createReader(payload)

	var obj readObject
	obj, _ = read(rd)

	if !reflect.DeepEqual(expected, obj) {
		t.Errorf("\nexpected: %#v\ngot: %#v\n", expected, obj)
		return
	}
}

func testReadError(t *testing.T, payload string) {
	var rd *bufio.Reader
	rd = createReader(payload)

	var err error
	_, err = read(rd)

	if err == nil {
		t.Errorf("\nexpected error to be non-nil\n")
		return
	}
}

func TestReadMessage(t *testing.T) {
	var expected = &readMessage{
		Subscription:   []byte("sub"),
		SubscriptionId: 1234,
		ReplyTo:        nil,
		Payload:        []byte("some message"),
	}

	testReadMatch(t, "msg sub 1234 12\r\nsome message\r\n", expected)
}

func TestReadMessageWithInvalidSubscriptionId(t *testing.T) {
	testReadError(t, "msg sub xxxx 12\r\nsome message\r\n")
}

func TestReadMessageWithInvalidByteCount(t *testing.T) {
	testReadError(t, "msg sub 1234 xx\r\nsome message\r\n")
}

func TestReadMessageWithEarlyEof(t *testing.T) {
	testReadError(t, "msg sub 1234 12\r\nsome message\r")
}

func TestReadOk(t *testing.T) {
	var expected = &readOk{}

	testReadMatch(t, "+ok\r\n", expected)
}

func TestReadErrWithoutPayload(t *testing.T) {
	var expected = &readErr{
		Payload: nil,
	}

	testReadMatch(t, "-err\r\n", expected)
}

func TestReadErrWithPayload(t *testing.T) {
	var expected = &readErr{
		Payload: []byte("foo"),
	}

	testReadMatch(t, "-err foo\r\n", expected)
}

func TestReadErrWithPayloadWithWhiteSpace(t *testing.T) {
	var expected = &readErr{
		Payload: []byte("foo bar qux"),
	}

	testReadMatch(t, "-err foo bar qux\r\n", expected)
}

func TestReadPing(t *testing.T) {
	var expected = &readPing{}

	testReadMatch(t, "ping\r\n", expected)
}

func TestReadPong(t *testing.T) {
	var expected = &readPong{}

	testReadMatch(t, "pong\r\n", expected)
}

func TestReadInfo(t *testing.T) {
	var expected = &readInfo{
		ServerId: "some id",
	}

	testReadMatch(t, "info {\"server_id\":\"some id\" }\r\n", expected)
}
