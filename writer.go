package nats

import (
	"fmt"
	"encoding/json"
	"bufio"
)

type writeObject interface {
	write(wr *bufio.Writer) (err error)
}

func write(wr *bufio.Writer, wobj writeObject) error {
	return wobj.write(wr)
}

func writeAndFlush(wr *bufio.Writer, wobj writeObject) error {
	var err error

	err = write(wr, wobj)
	if err != nil {
		return err
	}

	err = wr.Flush()
	if err != nil {
		return err
	}

	return nil
}

type writeConnect struct {
	Verbose  bool   `json:"verbose"`
	Pedantic bool   `json:"pedantic"`
	User     string `json:"user"`
	Pass     string `json:"pass"`
}

func (self *writeConnect) write(wr *bufio.Writer) error {
	var payload []byte
	var err error
	var protocol string

	payload, err = json.Marshal(self)
	if err != nil {
		return err
	}

	protocol = fmt.Sprintf("CONNECT %s\r\n", payload)

	_, err = wr.WriteString(protocol)
	if err != nil {
		return err
	}

	return nil
}

type writePing struct {
	// No content
}

func (self *writePing) write(wr *bufio.Writer) error {
	var err error
	var protocol string

	protocol = fmt.Sprintf("PING\r\n")

	_, err = wr.WriteString(protocol)
	if err != nil {
		return err
	}

	return nil
}

type writePong struct {
	// No content
}

func (self *writePong) write(wr *bufio.Writer) error {
	var err error
	var protocol string

	protocol = fmt.Sprintf("PONG\r\n")

	_, err = wr.WriteString(protocol)
	if err != nil {
		return err
	}

	return nil
}
