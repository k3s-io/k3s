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
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
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

type agentTunnel struct {
	client kubernetes.Interface
	cidrs  cidranger.Ranger
	ports  map[string]bool
	mode   string
}

// explicit interface check
var _ cidranger.RangerEntry = &podEntry{}

type podEntry struct {
	cidr    net.IPNet
	hostNet bool
}

func (p *podEntry) Network() net.IPNet {
	return p.cidr
}

func Setup(ctx context.Context, config *daemonconfig.Node, proxy proxy.Proxy) error {
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
		ports:  map[string]bool{},
		mode:   config.EgressSelectorMode,
	}

	apiServerReady := make(chan struct{})
	go func() {
		if err := util.WaitForAPIServerReady(ctx, config.AgentConfig.KubeConfigKubelet, util.DefaultAPIServerReadyTimeout); err != nil {
			logrus.Fatalf("Tunnel watches failed to wait for apiserver ready: %v", err)
		}
		close(apiServerReady)
	}()

	switch tunnel.mode {
	case daemonconfig.EgressSelectorModeCluster:
		// In Cluster mode, we allow the cluster CIDRs, and any connections to the node's IPs for pods using host network.
		tunnel.clusterAuth(config)
	case daemonconfig.EgressSelectorModePod:
		// In Pod mode, we watch pods assigned to this node, and allow their addresses, as well as ports used by containers with host network.
		go tunnel.watchPods(ctx, apiServerReady, config)
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

	wg := &sync.WaitGroup{}

	go tunnel.watchEndpoints(ctx, apiServerReady, wg, tlsConfig, proxy)

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

func (a *agentTunnel) clusterAuth(config *daemonconfig.Node) {
	// In Cluster mode, we add static entries for the Node IPs and Cluster CIDRs
	for _, ip := range config.AgentConfig.NodeIPs {
		if cidr, err := util.IPToIPNet(ip); err == nil {
			logrus.Infof("Tunnel authorizer adding Node IP %s", cidr)
			a.cidrs.Insert(&podEntry{cidr: *cidr})
		}
	}
	for _, cidr := range config.AgentConfig.ClusterCIDRs {
		logrus.Infof("Tunnel authorizer adding Cluster CIDR %s", cidr)
		a.cidrs.Insert(&podEntry{cidr: *cidr})
	}
}

// watchPods watches for pods assigned to this node, adding their IPs to the CIDR list.
// If the pod uses host network, we instead add the
func (a *agentTunnel) watchPods(ctx context.Context, apiServerReady <-chan struct{}, config *daemonconfig.Node) {
	for _, ip := range config.AgentConfig.NodeIPs {
		if cidr, err := util.IPToIPNet(ip); err == nil {
			logrus.Infof("Tunnel authorizer adding Node IP %s", cidr)
			a.cidrs.Insert(&podEntry{cidr: *cidr, hostNet: true})
		}
	}

	<-apiServerReady

	nodeName := os.Getenv("NODE_NAME")
	pods := a.client.CoreV1().Pods(metav1.NamespaceNone)
	fieldSelector := fields.Set{"spec.nodeName": nodeName}.String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (object runtime.Object, e error) {
			options.FieldSelector = fieldSelector
			return pods.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return pods.Watch(ctx, options)
		},
	}

	logrus.Infof("Tunnnel authorizer watching Pods")
	_, _, watch, done := toolswatch.NewIndexerInformerWatcher(lw, &v1.Pod{})

	defer func() {
		watch.Stop()
		<-done
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-watch.ResultChan():
			pod, ok := ev.Object.(*v1.Pod)
			if !ok {
				logrus.Errorf("Tunnel watch failed: event object not of type v1.Pod")
				continue
			}
			if pod.Spec.HostNetwork {
				for _, container := range pod.Spec.Containers {
					for _, port := range container.Ports {
						if port.Protocol == v1.ProtocolTCP {
							containerPort := fmt.Sprint(port.ContainerPort)
							if pod.DeletionTimestamp != nil {
								logrus.Debugf("Tunnel authorizer removing Node Port %s", containerPort)
								delete(a.ports, containerPort)
							} else {
								logrus.Debugf("Tunnel authorizer adding Node Port %s", containerPort)
								a.ports[containerPort] = true
							}
						}
					}
				}
			} else {
				for _, ip := range pod.Status.PodIPs {
					if cidr, err := util.IPStringToIPNet(ip.IP); err == nil {
						if pod.DeletionTimestamp != nil {
							logrus.Debugf("Tunnel authorizer removing Pod IP %s", cidr)
							a.cidrs.Remove(*cidr)
						} else {
							logrus.Debugf("Tunnel authorizer adding Pod IP %s", cidr)
							a.cidrs.Insert(&podEntry{cidr: *cidr})
						}
					}
				}
			}
		}
	}
}

// WatchEndpoints attempts to create tunnels to all supervisor addresses.  Once the
// apiserver is up, go into a watch loop, adding and removing tunnels as endpoints come
// and go from the cluster.
func (a *agentTunnel) watchEndpoints(ctx context.Context, apiServerReady <-chan struct{}, wg *sync.WaitGroup, tlsConfig *tls.Config, proxy proxy.Proxy) {
	// Attempt to connect to supervisors, storing their cancellation function for later when we
	// need to disconnect.
	disconnect := map[string]context.CancelFunc{}
	for _, address := range proxy.SupervisorAddresses() {
		if _, ok := disconnect[address]; !ok {
			disconnect[address] = a.connect(ctx, wg, address, tlsConfig)
		}
	}

	<-apiServerReady
	endpoints := a.client.CoreV1().Endpoints(metav1.NamespaceDefault)
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
					disconnect[address] = a.connect(ctx, nil, address, tlsConfig)
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

// authorized determines whether or not a dial request is authorized.
// Connections to the local kubelet ports are allowed.
// Connections to other IPs are allowed if they are contained in a CIDR managed by this node.
// All other requests are rejected.
func (a *agentTunnel) authorized(ctx context.Context, proto, address string) bool {
	logrus.Debugf("Tunnel authorizer checking dial request for %s", address)
	host, port, err := net.SplitHostPort(address)
	if err == nil {
		if proto == "tcp" && daemonconfig.KubeletReservedPorts[port] && (host == "127.0.0.1" || host == "::1") {
			return true
		}
		if ip := net.ParseIP(host); ip != nil {
			if nets, err := a.cidrs.ContainingNetworks(ip); err == nil && len(nets) > 0 {
				if p, ok := nets[0].(*podEntry); ok {
					if p.hostNet {
						return proto == "tcp" && a.ports[port]
					}
					return true
				}
				logrus.Debugf("Tunnel authorizer CIDR lookup returned unknown type for address %s", ip)
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
			}, func(_ context.Context, _ *remotedialer.Session) error {
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
