package control

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/remotedialer"
	"github.com/rancher/wrangler/pkg/kv"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/cmd/kube-apiserver/app"
)

func setupTunnel() http.Handler {
	tunnelServer := remotedialer.New(authorizer, remotedialer.DefaultErrorWriter)
	setupProxyDialer(tunnelServer)
	return tunnelServer
}

func setupProxyDialer(tunnelServer *remotedialer.Server) {
	app.DefaultProxyDialerFn = utilnet.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		_, port, _ := net.SplitHostPort(address)
		addr := "127.0.0.1"
		if port != "" {
			addr += ":" + port
		}
		nodeName, _ := kv.Split(address, ":")
		if tunnelServer.HasSession(nodeName) {
			return tunnelServer.Dial(nodeName, 15*time.Second, "tcp", addr)
		}
		var d net.Dialer
		return d.DialContext(ctx, network, address)
	})
}

func authorizer(req *http.Request) (clientKey string, authed bool, err error) {
	user, ok := request.UserFrom(req.Context())
	if !ok {
		return "", false, nil
	}

	if strings.HasPrefix(user.GetName(), "system:node:") {
		return strings.TrimPrefix(user.GetName(), "system:node:"), true, nil
	}

	return "", false, nil
}
