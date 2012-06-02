package nats

import (
	"testing"
	"fmt"
	"net"
)

var ErrWhatever = fmt.Errorf("whatever")

func TestDialMaxAttempts(t *testing.T) {
	d := DefaultDialer("address").(RetryingDialer)

	var i uint = 0

	// Fail every time
	d.f = func(addr string) (net.Conn, error) {
		i++
		return nil, ErrWhatever
	}

	// Don't sleep
	d.s = func(i uint) {
	}

	d.MaxAttempts = 2

	_, e := d.Dial()
	if e != ErrWhatever {
		t.Errorf("Expected: %#v, got: %#v", ErrWhatever, e)
		return
	}

	if i != d.MaxAttempts {
		t.Errorf("Expected: %#v, got: %#v", d.MaxAttempts, i)
		return
	}
}

func TestDialSuccess(t *testing.T) {
	d := DefaultDialer("address").(RetryingDialer)

	// Succeed
	d.f = func(addr string) (net.Conn, error) {
		n, _ := net.Pipe()
		return n, nil
	}

	_, e := d.Dial()
	if e != nil {
		t.Errorf("Error: %#v", e)
		return
	}
}
