package remotedialer

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type ConnectAuthorizer func(proto, address string) bool

func ClientConnect(ctx context.Context, wsURL string, headers http.Header, dialer *websocket.Dialer, auth ConnectAuthorizer, onConnect func(context.Context) error) {
	if err := connectToProxy(ctx, wsURL, headers, auth, dialer, onConnect); err != nil {
		logrus.WithError(err).Error("Remotedialer proxy error")
		time.Sleep(time.Duration(5) * time.Second)
	}
}

func connectToProxy(rootCtx context.Context, proxyURL string, headers http.Header, auth ConnectAuthorizer, dialer *websocket.Dialer, onConnect func(context.Context) error) error {
	logrus.WithField("url", proxyURL).Info("Connecting to proxy")

	if dialer == nil {
		dialer = &websocket.Dialer{}
	}
	ws, _, err := dialer.Dial(proxyURL, headers)
	if err != nil {
		logrus.WithError(err).Error("Failed to connect to proxy")
		return err
	}
	defer ws.Close()

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	if onConnect != nil {
		if err := onConnect(ctx); err != nil {
			return err
		}
	}

	session := NewClientSession(auth, ws)
	defer session.Close()

	result := make(chan error, 1)
	go func() {
		_, err = session.Serve(ctx)
		result <- err
	}()

	select {
	case <-ctx.Done():
		logrus.WithField("url", proxyURL).WithField("err", ctx.Err()).Info("Proxy done")
		return nil
	case err := <-result:
		return err
	}
}
