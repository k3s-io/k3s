package goStrongswanVici

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	DefaultReadTimeout = 15 * time.Second
)

// This object is not thread safe.
// if you want concurrent, you need create more clients.
type ClientConn struct {
	conn          net.Conn
	responseChan  chan segment
	eventHandlers map[string]func(response map[string]interface{})
	lastError     error

	// ReadTimeout specifies a time limit for requests made
	// by this client.
	ReadTimeout time.Duration

	lock sync.RWMutex
}

func (c *ClientConn) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	close(c.responseChan)
	c.lastError = io.ErrClosedPipe

	return c.conn.Close()
}

func NewClientConn(conn net.Conn) (client *ClientConn) {
	client = &ClientConn{
		conn:          conn,
		responseChan:  make(chan segment, 2),
		eventHandlers: map[string]func(response map[string]interface{}){},
		ReadTimeout:   DefaultReadTimeout,
	}
	go client.readThread()

	return client
}

// it dial from unix:///var/run/charon.vici
func NewClientConnFromDefaultSocket() (client *ClientConn, err error) {
	conn, err := net.Dial("unix", "/var/run/charon.vici")
	if err != nil {
		return
	}

	return NewClientConn(conn), nil
}

func (c *ClientConn) Request(apiname string, request map[string]interface{}) (response map[string]interface{}, err error) {
	err = writeSegment(c.conn, segment{
		typ:  stCMD_REQUEST,
		name: apiname,
		msg:  request,
	})
	if err != nil {
		fmt.Printf("error writing segment \n")

		return
	}

	outMsg := c.readResponse()
	c.lock.RLock()
	err = c.lastError
	if err != nil {
		c.lock.RUnlock()
		return nil, err
	}
	c.lock.RUnlock()

	if outMsg.typ != stCMD_RESPONSE {
		return nil, fmt.Errorf("[%s] response error %d", apiname, outMsg.typ)
	}

	return outMsg.msg, nil
}

func (c *ClientConn) readResponse() segment {
	select {
	case outMsg := <-c.responseChan:
		return outMsg
	case <-time.After(c.ReadTimeout):
		if c.lastError == nil {
			c.lock.Lock()
			c.lastError = fmt.Errorf("Timeout waiting for message response")
			c.lock.Unlock()
		}

		return segment{}
	}
}

func (c *ClientConn) RegisterEvent(name string, handler func(response map[string]interface{})) (err error) {
	c.lock.Lock()
	if c.eventHandlers[name] != nil {
		c.lock.Unlock()
		return fmt.Errorf("[event %s] register a event twice.", name)
	}

	c.eventHandlers[name] = handler
	err = writeSegment(c.conn, segment{
		typ:  stEVENT_REGISTER,
		name: name,
	})
	if err != nil {
		delete(c.eventHandlers, name)
		c.lock.Unlock()

		return
	}
	c.lock.Unlock()
	outMsg := c.readResponse()
	// fmt.Printf("registerEvent %#v\n", outMsg)
	c.lock.Lock()
	lastError := c.lastError

	if lastError != nil {
		delete(c.eventHandlers, name)
		c.lock.Unlock()

		return err
	}

	if outMsg.typ != stEVENT_CONFIRM {
		delete(c.eventHandlers, name)
		c.lock.Unlock()

		return fmt.Errorf("[event %s] response error %d", name, outMsg.typ)
	}
	c.lock.Unlock()

	return nil
}

func (c *ClientConn) UnregisterEvent(name string) (err error) {
	err = writeSegment(c.conn, segment{
		typ:  stEVENT_UNREGISTER,
		name: name,
	})
	if err != nil {
		return
	}

	outMsg := c.readResponse()
	// fmt.Printf("UnregisterEvent %#v\n", outMsg)
	c.lock.Lock()
	if c.lastError != nil {
		c.lock.Unlock()

		return c.lastError
	}
	c.lock.Unlock()

	if outMsg.typ != stEVENT_CONFIRM {
		return fmt.Errorf("[event %s] response error %d", name, outMsg.typ)
	}

	c.lock.Lock()
	delete(c.eventHandlers, name)
	c.lock.Unlock()

	return nil
}

func (c *ClientConn) readThread() {
	for {
		outMsg, err := readSegment(c.conn)
		if err != nil {
			c.lock.Lock()
			c.lastError = err
			c.lock.Unlock()

			return
		}

		switch outMsg.typ {
		case stCMD_RESPONSE, stEVENT_CONFIRM:
			c.responseChan <- outMsg
		case stEVENT:
			c.lock.Lock()
			handler := c.eventHandlers[outMsg.name]
			c.lock.Unlock()

			if handler != nil {
				handler(outMsg.msg)
			}
		default:
			c.lock.Lock()
			c.lastError = fmt.Errorf("[Client.readThread] unknow msg type %d", outMsg.typ)
			c.lock.Unlock()

			return
		}
	}
}
