package remotedialer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type Session struct {
	sync.Mutex

	nextConnID       int64
	clientKey        string
	sessionKey       int64
	conn             *wsConn
	conns            map[int64]*connection
	remoteClientKeys map[string]map[int]bool
	auth             ConnectAuthorizer
	pingCancel       context.CancelFunc
	pingWait         sync.WaitGroup
	dialer           Dialer
	client           bool
}

// PrintTunnelData No tunnel logging by default
var PrintTunnelData bool

func init() {
	if os.Getenv("CATTLE_TUNNEL_DATA_DEBUG") == "true" {
		PrintTunnelData = true
	}
}

func NewClientSession(auth ConnectAuthorizer, conn *websocket.Conn) *Session {
	return &Session{
		clientKey: "client",
		conn:      newWSConn(conn),
		conns:     map[int64]*connection{},
		auth:      auth,
		client:    true,
	}
}

func newSession(sessionKey int64, clientKey string, conn *websocket.Conn) *Session {
	return &Session{
		nextConnID:       1,
		clientKey:        clientKey,
		sessionKey:       sessionKey,
		conn:             newWSConn(conn),
		conns:            map[int64]*connection{},
		remoteClientKeys: map[string]map[int]bool{},
	}
}

func (s *Session) startPings() {
	ctx, cancel := context.WithCancel(context.Background())
	s.pingCancel = cancel
	s.pingWait.Add(1)

	go func() {
		defer s.pingWait.Done()

		t := time.NewTicker(PingWriteInterval)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.conn.Lock()
				if err := s.conn.conn.WriteControl(websocket.PingMessage, []byte(""), time.Now().Add(time.Second)); err != nil {
					logrus.WithError(err).Error("Error writing ping")
				}
				logrus.Debug("Wrote ping")
				s.conn.Unlock()
			}
		}
	}()
}

func (s *Session) stopPings() {
	if s.pingCancel == nil {
		return
	}

	s.pingCancel()
	s.pingWait.Wait()
}

func (s *Session) Serve() (int, error) {
	if s.client {
		s.startPings()
	}

	for {
		msType, reader, err := s.conn.NextReader()
		if err != nil {
			return 400, err
		}

		if msType != websocket.BinaryMessage {
			return 400, errWrongMessageType
		}

		if err := s.serveMessage(reader); err != nil {
			return 500, err
		}
	}
}

func (s *Session) serveMessage(reader io.Reader) error {
	message, err := newServerMessage(reader)
	if err != nil {
		return err
	}

	if PrintTunnelData {
		logrus.Debug("REQUEST ", message)
	}

	if message.messageType == Connect {
		if s.auth == nil || !s.auth(message.proto, message.address) {
			return errors.New("connect not allowed")
		}
		s.clientConnect(message)
		return nil
	}

	s.Lock()
	if message.messageType == AddClient && s.remoteClientKeys != nil {
		err := s.addRemoteClient(message.address)
		s.Unlock()
		return err
	} else if message.messageType == RemoveClient {
		err := s.removeRemoteClient(message.address)
		s.Unlock()
		return err
	}
	conn := s.conns[message.connID]
	s.Unlock()

	if conn == nil {
		if message.messageType == Data {
			err := fmt.Errorf("connection not found %s/%d/%d", s.clientKey, s.sessionKey, message.connID)
			newErrorMessage(message.connID, err).WriteTo(s.conn)
		}
		return nil
	}

	switch message.messageType {
	case Data:
		if _, err := io.Copy(conn.tunnelWriter(), message); err != nil {
			s.closeConnection(message.connID, err)
		}
	case Error:
		s.closeConnection(message.connID, message.Err())
	}

	return nil
}

func parseAddress(address string) (string, int, error) {
	parts := strings.SplitN(address, "/", 2)
	if len(parts) != 2 {
		return "", 0, errors.New("not / separated")
	}
	v, err := strconv.Atoi(parts[1])
	return parts[0], v, err
}

func (s *Session) addRemoteClient(address string) error {
	clientKey, sessionKey, err := parseAddress(address)
	if err != nil {
		return fmt.Errorf("invalid remote Session %s: %v", address, err)
	}

	keys := s.remoteClientKeys[clientKey]
	if keys == nil {
		keys = map[int]bool{}
		s.remoteClientKeys[clientKey] = keys
	}
	keys[int(sessionKey)] = true

	if PrintTunnelData {
		logrus.Debugf("ADD REMOTE CLIENT %s, SESSION %d", address, s.sessionKey)
	}

	return nil
}

func (s *Session) removeRemoteClient(address string) error {
	clientKey, sessionKey, err := parseAddress(address)
	if err != nil {
		return fmt.Errorf("invalid remote Session %s: %v", address, err)
	}

	keys := s.remoteClientKeys[clientKey]
	delete(keys, int(sessionKey))
	if len(keys) == 0 {
		delete(s.remoteClientKeys, clientKey)
	}

	if PrintTunnelData {
		logrus.Debugf("REMOVE REMOTE CLIENT %s, SESSION %d", address, s.sessionKey)
	}

	return nil
}

func (s *Session) closeConnection(connID int64, err error) {
	s.Lock()
	conn := s.conns[connID]
	delete(s.conns, connID)
	if PrintTunnelData {
		logrus.Debugf("CONNECTIONS %d %d", s.sessionKey, len(s.conns))
	}
	s.Unlock()

	if conn != nil {
		conn.tunnelClose(err)
	}
}

func (s *Session) clientConnect(message *message) {
	conn := newConnection(message.connID, s, message.proto, message.address)

	s.Lock()
	s.conns[message.connID] = conn
	if PrintTunnelData {
		logrus.Debugf("CONNECTIONS %d %d", s.sessionKey, len(s.conns))
	}
	s.Unlock()

	go clientDial(s.dialer, conn, message)
}

func (s *Session) serverConnect(deadline time.Duration, proto, address string) (net.Conn, error) {
	connID := atomic.AddInt64(&s.nextConnID, 1)
	conn := newConnection(connID, s, proto, address)

	s.Lock()
	s.conns[connID] = conn
	if PrintTunnelData {
		logrus.Debugf("CONNECTIONS %d %d", s.sessionKey, len(s.conns))
	}
	s.Unlock()

	_, err := s.writeMessage(newConnect(connID, deadline, proto, address))
	if err != nil {
		s.closeConnection(connID, err)
		return nil, err
	}

	return conn, err
}

func (s *Session) writeMessage(message *message) (int, error) {
	if PrintTunnelData {
		logrus.Debug("WRITE ", message)
	}
	return message.WriteTo(s.conn)
}

func (s *Session) Close() {
	s.Lock()
	defer s.Unlock()

	s.stopPings()

	for _, connection := range s.conns {
		connection.tunnelClose(errors.New("tunnel disconnect"))
	}

	s.conns = map[int64]*connection{}
}

func (s *Session) sessionAdded(clientKey string, sessionKey int64) {
	client := fmt.Sprintf("%s/%d", clientKey, sessionKey)
	_, err := s.writeMessage(newAddClient(client))
	if err != nil {
		s.conn.conn.Close()
	}
}

func (s *Session) sessionRemoved(clientKey string, sessionKey int64) {
	client := fmt.Sprintf("%s/%d", clientKey, sessionKey)
	_, err := s.writeMessage(newRemoveClient(client))
	if err != nil {
		s.conn.conn.Close()
	}
}
