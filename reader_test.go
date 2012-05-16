package protocol

import (
	"reflect"
	"bufio"
	"strings"
	"testing"
	"net/textproto"
)

// Create a textproto.Reader
func createReader(payload string) *textproto.Reader {
	sio := strings.NewReader(payload)
	bio := bufio.NewReader(sio)
	return textproto.NewReader(bio)
}

func testMatch(t *testing.T, payload string, expected readObject) {
	var rd *textproto.Reader
	rd = createReader(payload)

	var obj readObject
	obj, _ = read(rd)

	if !reflect.DeepEqual(expected, obj) {
		t.Errorf("\nexpected: %#v\ngot: %#v\n", expected, obj)
	}
}

func testError(t *testing.T, payload string) {
	var rd *textproto.Reader
	rd = createReader(payload)

	var err error
	_, err = read(rd)

	if err == nil {
		t.Errorf("\nexpected error to be non-nil\n")
	}
}

func TestReadMessage(t *testing.T) {
	var expected = &readMessage{
		Subscription:   []byte("sub"),
		SubscriptionId: 1234,
		ReplyTo:        nil,
		Payload:        []byte("some message"),
	}

	testMatch(t, "msg sub 1234 12\r\nsome message\r\n", expected)
}

func TestReadMessageWithInvalidSubscriptionId(t *testing.T) {
	testError(t, "msg sub xxxx 12\r\nsome message\r\n")
}

func TestReadMessageWithInvalidByteCount(t *testing.T) {
	testError(t, "msg sub 1234 xx\r\nsome message\r\n")
}

func TestReadMessageWithEarlyEof(t *testing.T) {
	testError(t, "msg sub 1234 12\r\nsome message\r")
}

func TestReadOk(t *testing.T) {
	var expected = &readOk{}

	testMatch(t, "+ok\r\n", expected)
}

func TestReadErrWithoutPayload(t *testing.T) {
	var expected = &readErr{
		Payload: nil,
	}

	testMatch(t, "-err\r\n", expected)
}

func TestReadErrWithPayload(t *testing.T) {
	var expected = &readErr{
		Payload: []byte("foo"),
	}

	testMatch(t, "-err foo\r\n", expected)
}

func TestReadErrWithPayloadWithWhiteSpace(t *testing.T) {
	var expected = &readErr{
		Payload: []byte("foo bar qux"),
	}

	testMatch(t, "-err foo bar qux\r\n", expected)
}

func TestReadPing(t *testing.T) {
	var expected = &readPing{}

	testMatch(t, "ping\r\n", expected)
}

func TestReadPong(t *testing.T) {
	var expected = &readPong{}

	testMatch(t, "pong\r\n", expected)
}

func TestReadInfo(t *testing.T) {
	var expected = &readInfo{
		ServerId: "some id",
	}

	testMatch(t, "info {\"server_id\":\"some id\" }\r\n", expected)
}
