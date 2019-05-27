package tunnel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ports = map[string]bool{
		"10250": true,
		"10010": true,
	}
)

func Setup(config *config.Node) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", config.AgentConfig.KubeConfig)
	if err != nil {
		return err
	}

	transportConfig, err := restConfig.TransportConfig()
	if err != nil {
		return err
	}

	wsURL := fmt.Sprintf("wss://%s/v1-k3s/connect", config.ServerAddress)
	headers := map[string][]string{
		"X-K3s-NodeName": {config.AgentConfig.NodeName},
	}
	ws := &websocket.Dialer{}

	if len(config.CACerts) > 0 {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(config.CACerts)
		ws.TLSClientConfig = &tls.Config{
			RootCAs: pool,
		}
	}

	if transportConfig.Username != "" {
		auth := transportConfig.Username + ":" + transportConfig.Password
		auth = base64.StdEncoding.EncodeToString([]byte(auth))
		headers["Authorization"] = []string{"Basic " + auth}
	}

	once := sync.Once{}
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		for {
			logrus.Infof("Connecting to %s", wsURL)
			remotedialer.ClientConnect(wsURL, http.Header(headers), ws, func(proto, address string) bool {
				host, port, err := net.SplitHostPort(address)
				return err == nil && proto == "tcp" && ports[port] && host == "127.0.0.1"
			}, func(_ context.Context) error {
				once.Do(wg.Done)
				return nil
			})
			time.Sleep(5 * time.Second)
		}
	}()

	wg.Wait()
	return nil
}
