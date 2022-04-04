package tunnel

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"reflect"
	"sync"

	"github.com/gorilla/websocket"
	agentconfig "github.com/k3s-io/k3s/pkg/agent/config"
	"github.com/k3s-io/k3s/pkg/agent/proxy"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
	"github.com/yl2chen/cidranger"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	toolswatch "k8s.io/client-go/tools/watch"
)

var (
	ports = map[string]bool{
		"10250": true,
		"10010": true,
	}
)

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

	tunnel := &agentTunnel{
		client: client,
		cidrs:  cidranger.NewPCTrieRanger(),
	}

	// The loadbalancer is only disabled when there is a local apiserver.  Servers without a local
	// apiserver load-balance to themselves initially, then switch over to an apiserver node as soon
	// as we get some addresses from the code below.
	if proxy.IsSupervisorLBEnabled() && proxy.SupervisorURL() != "" {
		logrus.Info("Getting list of apiserver endpoints from server")
		// If not running an apiserver locally, try to get a list of apiservers from the server we're
		// connecting to. If that fails, fall back to querying the endpoints list from Kubernetes. This
		// fallback requires that the server we're joining be running an apiserver, but is the only safe
		// thing to do if its supervisor is down-level and can't provide us with an endpoint list.
		if addresses := agentconfig.APIServers(ctx, config, proxy); len(addresses) > 0 {
			proxy.SetSupervisorDefault(addresses[0])
			proxy.Update(addresses)
		} else {
			if endpoint, _ := client.CoreV1().Endpoints("default").Get(ctx, "kubernetes", metav1.GetOptions{}); endpoint != nil {
				if addresses := util.GetAddresses(endpoint); len(addresses) > 0 {
					proxy.Update(addresses)
				}
			}
		}
	}

	// Attempt to connect to supervisors, storing their cancellation function for later when we
	// need to disconnect.
	disconnect := map[string]context.CancelFunc{}
	wg := &sync.WaitGroup{}
	for _, address := range proxy.SupervisorAddresses() {
		if _, ok := disconnect[address]; !ok {
			disconnect[address] = tunnel.connect(ctx, wg, address, tlsConfig)
		}
	}

	// Once the apiserver is up, go into a watch loop, adding and removing tunnels as endpoints come
	// and go from the cluster.
	go func() {
		if err := util.WaitForAPIServerReady(ctx, config.AgentConfig.KubeConfigKubelet, util.DefaultAPIServerReadyTimeout); err != nil {
			logrus.Warnf("Tunnel endpoint watch failed to wait for apiserver ready: %v", err)
		}

		endpoints := client.CoreV1().Endpoints(metav1.NamespaceDefault)
		fieldSelector := fields.Set{metav1.ObjectNameField: "kubernetes"}.String()
		lw := &cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (object runtime.Object, e error) {
				options.FieldSelector = fieldSelector
				return endpoints.List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
				options.FieldSelector = fieldSelector
				return endpoints.Watch(ctx, options)
			},
		}

		_, _, watch, done := toolswatch.NewIndexerInformerWatcher(lw, &v1.Endpoints{})

		defer func() {
			watch.Stop()
			<-done
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-watch.ResultChan():
				endpoint, ok := ev.Object.(*v1.Endpoints)
				if !ok {
					logrus.Errorf("Tunnel watch failed: event object not of type v1.Endpoints")
					continue
				}

				newAddresses := util.GetAddresses(endpoint)
				if reflect.DeepEqual(newAddresses, proxy.SupervisorAddresses()) {
					continue
				}
				proxy.Update(newAddresses)

				validEndpoint := map[string]bool{}

				for _, address := range proxy.SupervisorAddresses() {
					validEndpoint[address] = true
					if _, ok := disconnect[address]; !ok {
						disconnect[address] = tunnel.connect(ctx, nil, address, tlsConfig)
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
	}()

	wait := make(chan int, 1)
	go func() {
		wg.Wait()
		wait <- 0
	}()

	select {
	case <-ctx.Done():
		logrus.Error("Tunnel context canceled while waiting for connection")
		return ctx.Err()
	case <-wait:
	}

	return nil
}

type agentTunnel struct {
	client kubernetes.Interface
	cidrs  cidranger.Ranger
}

// authorized determines whether or not a dial request is authorized.
// Connections to the local kubelet ports are allowed.
// Connections to other IPs are allowed if they are contained in a CIDR managed by this node.
// All other requests are rejected.
func (a *agentTunnel) authorized(ctx context.Context, proto, address string) bool {
	logrus.Debugf("Tunnel authorizer checking dial request for %s", address)
	host, port, err := net.SplitHostPort(address)
	if err == nil {
		if proto == "tcp" && ports[port] && (host == "127.0.0.1" || host == "::1") {
			return true
		}
		if ip := net.ParseIP(host); ip != nil {
			// lazy populate the cidrs from the node object
			if a.cidrs.Len() == 0 {
				logrus.Debugf("Tunnel authorizer getting Pod CIDRs for %s", os.Getenv("NODE_NAME"))
				node, err := a.client.CoreV1().Nodes().Get(ctx, os.Getenv("NODE_NAME"), metav1.GetOptions{})
				if err != nil {
					logrus.Warnf("Tunnel authorizer failed to get Pod CIDRs: %v", err)
					return false
				}
				for _, cidr := range node.Spec.PodCIDRs {
					if _, n, err := net.ParseCIDR(cidr); err == nil {
						logrus.Infof("Tunnel authorizer added Pod CIDR %s", cidr)
						a.cidrs.Insert(cidranger.NewBasicRangerEntry(*n))
					}
				}
			}
			if nets, err := a.cidrs.ContainingNetworks(ip); err == nil && len(nets) > 0 {
				return true
			}
		}
	}
	return false
}

// connect initiates a connection to the remotedialer server. Incoming dial requests from
// the server will be checked by the authorizer function prior to being fulfilled.
func (a *agentTunnel) connect(rootCtx context.Context, waitGroup *sync.WaitGroup, address string, tlsConfig *tls.Config) context.CancelFunc {
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
				return a.authorized(rootCtx, proto, address)
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
