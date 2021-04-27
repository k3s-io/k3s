package remotedialer

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	Data messageType = iota + 1
	Connect
	Error
	AddClient
	RemoveClient
)

var (
	idCounter int64
)

func init() {
	r := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	idCounter = r.Int63()
}

type messageType int64

type message struct {
	id          int64
	err         error
	connID      int64
	deadline    int64
	messageType messageType
	bytes       []byte
	body        io.Reader
	proto       string
	address     string
}

func nextid() int64 {
	return atomic.AddInt64(&idCounter, 1)
}

func newMessage(connID int64, deadline int64, bytes []byte) *message {
	return &message{
		id:          nextid(),
		connID:      connID,
		deadline:    deadline,
		messageType: Data,
		bytes:       bytes,
	}
}

func newConnect(connID int64, deadline time.Duration, proto, address string) *message {
	return &message{
		id:          nextid(),
		connID:      connID,
		deadline:    deadline.Nanoseconds() / 1000000,
		messageType: Connect,
		bytes:       []byte(fmt.Sprintf("%s/%s", proto, address)),
		proto:       proto,
		address:     address,
	}
}

func newErrorMessage(connID int64, err error) *message {
	return &message{
		id:          nextid(),
		err:         err,
		connID:      connID,
		messageType: Error,
		bytes:       []byte(err.Error()),
	}
}

func newAddClient(client string) *message {
	return &message{
		id:          nextid(),
		messageType: AddClient,
		address:     client,
		bytes:       []byte(client),
	}
}

func newRemoveClient(client string) *message {
	return &message{
		id:          nextid(),
		messageType: RemoveClient,
		address:     client,
		bytes:       []byte(client),
	}
}

func newServerMessage(reader io.Reader) (*message, error) {
	buf := bufio.NewReader(reader)

	id, err := binary.ReadVarint(buf)
	if err != nil {
		return nil, err
	}

	connID, err := binary.ReadVarint(buf)
	if err != nil {
		return nil, err
	}

	mType, err := binary.ReadVarint(buf)
	if err != nil {
		return nil, err
	}

	m := &message{
		id:          id,
		messageType: messageType(mType),
		connID:      connID,
		body:        buf,
	}

	if m.messageType == Data || m.messageType == Connect {
		deadline, err := binary.ReadVarint(buf)
		if err != nil {
			return nil, err
		}
		m.deadline = deadline
	}

	if m.messageType == Connect {
		bytes, err := ioutil.ReadAll(io.LimitReader(buf, 100))
		if err != nil {
			return nil, err
		}
		parts := strings.SplitN(string(bytes), "/", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("failed to parse connect address")
		}
		m.proto = parts[0]
		m.address = parts[1]
		m.bytes = bytes
	} else if m.messageType == AddClient || m.messageType == RemoveClient {
		bytes, err := ioutil.ReadAll(io.LimitReader(buf, 100))
		if err != nil {
			return nil, err
		}
		m.address = string(bytes)
		m.bytes = bytes
	}

	return m, nil
}

func (m *message) Err() error {
	if m.err != nil {
		return m.err
	}
	bytes, err := ioutil.ReadAll(io.LimitReader(m.body, 100))
	if err != nil {
		return err
	}

	str := string(bytes)
	if str == "EOF" {
		m.err = io.EOF
	} else {
		m.err = errors.New(str)
	}
	return m.err
}

func (m *message) Bytes() []byte {
	return append(m.header(), m.bytes...)
}

func (m *message) header() []byte {
	buf := make([]byte, 24)
	offset := 0
	offset += binary.PutVarint(buf[offset:], m.id)
	offset += binary.PutVarint(buf[offset:], m.connID)
	offset += binary.PutVarint(buf[offset:], int64(m.messageType))
	if m.messageType == Data || m.messageType == Connect {
		offset += binary.PutVarint(buf[offset:], m.deadline)
	}
	return buf[:offset]
}

func (m *message) Read(p []byte) (int, error) {
	return m.body.Read(p)
}

func (m *message) WriteTo(wsConn *wsConn) (int, error) {
	err := wsConn.WriteMessage(websocket.BinaryMessage, m.Bytes())
	return len(m.bytes), err
}

func (m *message) String() string {
	switch m.messageType {
	case Data:
		if m.body == nil {
			return fmt.Sprintf("%d DATA         [%d]: %d bytes: %s", m.id, m.connID, len(m.bytes), string(m.bytes))
		}
		return fmt.Sprintf("%d DATA         [%d]: buffered", m.id, m.connID)
	case Error:
		return fmt.Sprintf("%d ERROR        [%d]: %s", m.id, m.connID, m.Err())
	case Connect:
		return fmt.Sprintf("%d CONNECT      [%d]: %s/%s deadline %d", m.id, m.connID, m.proto, m.address, m.deadline)
	case AddClient:
		return fmt.Sprintf("%d ADDCLIENT    [%s]", m.id, m.address)
	case RemoveClient:
		return fmt.Sprintf("%d REMOVECLIENT [%s]", m.id, m.address)
	}
	return fmt.Sprintf("%d UNKNOWN[%d]: %d", m.id, m.connID, m.messageType)
}
