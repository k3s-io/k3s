package remotedialer

import (
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type wsConn struct {
	sync.Mutex
	conn *websocket.Conn
}

func newWSConn(conn *websocket.Conn) *wsConn {
	w := &wsConn{
		conn: conn,
	}
	w.setupDeadline()
	return w
}

func (w *wsConn) WriteMessage(messageType int, data []byte) error {
	w.Lock()
	defer w.Unlock()
	w.conn.SetWriteDeadline(time.Now().Add(PingWaitDuration))
	return w.conn.WriteMessage(messageType, data)
}

func (w *wsConn) NextReader() (int, io.Reader, error) {
	return w.conn.NextReader()
}

func (w *wsConn) setupDeadline() {
	w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration))
	w.conn.SetPingHandler(func(string) error {
		w.Lock()
		w.conn.WriteControl(websocket.PongMessage, []byte(""), time.Now().Add(time.Second))
		w.Unlock()
		return w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration))
	})
	w.conn.SetPongHandler(func(string) error {
		return w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration))
	})

}
