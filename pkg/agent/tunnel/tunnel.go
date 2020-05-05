package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rancher/k3s/pkg/agent/proxy"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/version"
	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	watchtypes "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ports = map[string]bool{
		"10250": true,
		"10010": true,
	}
)

func getAddresses(endpoint *v1.Endpoints) []string {
	serverAddresses := []string{}
	if endpoint == nil {
		return serverAddresses
	}
	for _, subset := range endpoint.Subsets {
		var port string
		if len(subset.Ports) > 0 {
			port = strconv.Itoa(int(subset.Ports[0].Port))
		}
		if port == "" {
			port = "443"
		}
		for _, address := range subset.Addresses {
			serverAddresses = append(serverAddresses, net.JoinHostPort(address.IP, port))
		}
	}
	return serverAddresses
}

func Setup(ctx context.Context, config *config.Node, proxy proxy.Proxy) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", config.AgentConfig.KubeConfigK3sController)
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	nodeRestConfig, err := clientcmd.BuildConfigFromFlags("", config.AgentConfig.KubeConfigKubelet)
	if err != nil {
		return err
	}

	tlsConfig, err := rest.TLSConfigFor(nodeRestConfig)
	if err != nil {
		return err
	}

	endpoint, _ := client.CoreV1().Endpoints("default").Get(ctx, "kubernetes", metav1.GetOptions{})
	if endpoint != nil {
		proxy.Update(getAddresses(endpoint))
	}

	disconnect := map[string]context.CancelFunc{}

	wg := &sync.WaitGroup{}
	for _, address := range proxy.SupervisorAddresses() {
		if _, ok := disconnect[address]; !ok {
			disconnect[address] = connect(ctx, wg, address, tlsConfig)
		}
	}

	go func() {
	connect:
		for {
			time.Sleep(5 * time.Second)
			watch, err := client.CoreV1().Endpoints("default").Watch(ctx, metav1.ListOptions{
				FieldSelector:   fields.Set{"metadata.name": "kubernetes"}.String(),
				ResourceVersion: "0",
			})
			if err != nil {
				logrus.Errorf("Unable to watch for tunnel endpoints: %v", err)
				continue connect
			}
		watching:
			for {
				select {
				case ev, ok := <-watch.ResultChan():
					if !ok || ev.Type == watchtypes.Error {
						if ok {
							logrus.Errorf("Tunnel endpoint watch channel closed: %v", ev)
						}
						watch.Stop()
						continue connect
					}
					endpoint, ok := ev.Object.(*v1.Endpoints)
					if !ok {
						logrus.Errorf("Tunnel could not case event object to endpoint: %v", ev)
						continue watching
					}

					newAddresses := getAddresses(endpoint)
					if reflect.DeepEqual(newAddresses, proxy.SupervisorAddresses()) {
						continue watching
					}
					proxy.Update(newAddresses)

					validEndpoint := map[string]bool{}

					for _, address := range proxy.SupervisorAddresses() {
						validEndpoint[address] = true
						if _, ok := disconnect[address]; !ok {
							disconnect[address] = connect(ctx, nil, address, tlsConfig)
						}
					}

					for address, cancel := range disconnect {
						if !validEndpoint[address] {
							cancel()
							delete(disconnect, address)
							logrus.Infof("Stopped tunnel to %s", address)
						}
					}
				}
			}
		}
	}()

	wait := make(chan int, 1)
	go func() {
		wg.Wait()
		wait <- 0
	}()

	select {
	case <-ctx.Done():
		logrus.Error("tunnel context canceled while waiting for connection")
		return ctx.Err()
	case <-wait:
	}

	return nil
}

func connect(rootCtx context.Context, waitGroup *sync.WaitGroup, address string, tlsConfig *tls.Config) context.CancelFunc {
	wsURL := fmt.Sprintf("wss://%s/v1-"+version.Program+"/connect", address)
	ws := &websocket.Dialer{
		TLSClientConfig: tlsConfig,
	}

	once := sync.Once{}
	if waitGroup != nil {
		waitGroup.Add(1)
	}

	ctx, cancel := context.WithCancel(rootCtx)

	go func() {
		for {
			remotedialer.ClientConnect(ctx, wsURL, nil, ws, func(proto, address string) bool {
				host, port, err := net.SplitHostPort(address)
				return err == nil && proto == "tcp" && ports[port] && host == "127.0.0.1"
			}, func(_ context.Context) error {
				if waitGroup != nil {
					once.Do(waitGroup.Done)
				}
				return nil
			})

			if ctx.Err() != nil {
				if waitGroup != nil {
					once.Do(waitGroup.Done)
				}
				return
			}
		}
	}()

	return cancel
}
