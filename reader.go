package nats

import (
	"errors"
	"bytes"
	"encoding/json"
	"bufio"
	"regexp"
	"strconv"
)

var (
	ErrLineTooLong   = errors.New("reader: line too long")
	ErrUnknownObject = errors.New("reader: unknown object")
	ErrInvalidObject = errors.New("reader: invalid object")
)

var (
	nonSpaceRegexp = regexp.MustCompile("\\S+")
)

type readObject interface {
	read(line []byte, rd *bufio.Reader) (err error)
}

type readMessage struct {
	Subscription   []byte
	SubscriptionId uint
	ReplyTo        []byte
	Payload        []byte
}

func (self *readMessage) read(line []byte, rd *bufio.Reader) (err error) {
	var chunks [][]byte

	chunks = nonSpaceRegexp.FindAll(line, -1)

	if len(chunks) < 4 {
		return ErrInvalidObject
	}

	// Skip +MSG
	idx := 1
	self.Subscription = chunks[idx]

	idx += 1
	sid, err := strconv.ParseUint(string(chunks[idx]), 10, 0)
	if err != nil {
		return err
	}

	self.SubscriptionId = uint(sid)

	if len(chunks) == 5 {
		idx += 1
		self.ReplyTo = chunks[idx]
	}

	idx += 1
	bytes, err := strconv.ParseUint(string(chunks[idx]), 10, 0)
	if err != nil {
		return err
	}

	self.Payload = make([]byte, bytes+2)

	// Read until self.Payload is filled
	var target []byte = self.Payload
	for len(target) > 0 {
		n, err := rd.Read(target)
		if err != nil {
			return err
		}

		target = target[n:]
	}

	// Trim to actual payload size, removing CRLF
	self.Payload = self.Payload[:bytes]

	return
}

type readOk struct {
	// No content
}

func (self *readOk) read(line []byte, rd *bufio.Reader) (err error) {
	return
}

type readErr struct {
	Payload []byte
}

func (self *readErr) read(line []byte, rd *bufio.Reader) (err error) {
	var index [][]int

	index = nonSpaceRegexp.FindAllIndex(line, 2)

	if len(index) == 2 {
		self.Payload = line[index[1][0]:]
	}

	return
}

type readPing struct {
	// No content
}

func (self *readPing) read(line []byte, rd *bufio.Reader) (err error) {
	return
}

type readPong struct {
	// No content
}

func (self *readPong) read(line []byte, rd *bufio.Reader) (err error) {
	return
}

type readInfo struct {
	ServerId     string `json:"server_id"`
	Version      string `json:"version"`
	AuthRequired bool   `json:"auth_required"`
	SslRequired  bool   `json:"ssl_required"`
	MaxPayload   int64  `json:"max_payload"`
}

func (self *readInfo) read(line []byte, rd *bufio.Reader) (err error) {
	var index [][]int

	index = nonSpaceRegexp.FindAllIndex(line, 2)

	if len(index) == 2 {
		err = json.Unmarshal(line[index[1][0]:], &self)
		if err != nil {
			return err
		}
	}

	return
}

func read(rd *bufio.Reader) (readObject, error) {
	var line []byte
	var more bool
	var err error

	line, more, err = rd.ReadLine()

	if err != nil {
		return nil, err
	}

	if more {
		return nil, ErrLineTooLong
	}

	var head []byte = nonSpaceRegexp.Find(line)
	var obj readObject

	switch string(bytes.ToLower(head)) {
	case "msg":
		obj = new(readMessage)
	case "+ok":
		obj = new(readOk)
	case "-err":
		obj = new(readErr)
	case "ping":
		obj = new(readPing)
	case "pong":
		obj = new(readPong)
	case "info":
		obj = new(readInfo)
	default:
		return nil, ErrUnknownObject
	}

	err = obj.read(line, rd)
	if err != nil {
		return nil, err
	}

	return obj, nil
}
