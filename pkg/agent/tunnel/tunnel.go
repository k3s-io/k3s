package tunnel

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	agentconfig "github.com/k3s-io/k3s/pkg/agent/config"
	"github.com/k3s-io/k3s/pkg/agent/proxy"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
	"github.com/yl2chen/cidranger"
	authorizationv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	toolswatch "k8s.io/client-go/tools/watch"
	"k8s.io/kubernetes/pkg/cluster/ports"
)

var (
	endpointDebounceDelay = time.Second
	defaultDialer         = net.Dialer{}
)

type agentTunnel struct {
	client      kubernetes.Interface
	cidrs       cidranger.Ranger
	ports       map[string]bool
	mode        string
	kubeletAddr string
	kubeletPort string
	startTime   time.Time
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
	client, err := util.GetClientSet(config.AgentConfig.KubeConfigK3sController)
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
		client:      client,
		cidrs:       cidranger.NewPCTrieRanger(),
		ports:       map[string]bool{},
		mode:        config.EgressSelectorMode,
		kubeletAddr: config.AgentConfig.ListenAddress,
		kubeletPort: fmt.Sprint(ports.KubeletPort),
		startTime:   time.Now().Truncate(time.Second),
	}

	apiServerReady := make(chan struct{})
	go func() {
		if err := util.WaitForAPIServerReady(ctx, config.AgentConfig.KubeConfigKubelet, util.DefaultAPIServerReadyTimeout); err != nil {
			logrus.Fatalf("Tunnel watches failed to wait for apiserver ready: %v", err)
		}
		if err := util.WaitForRBACReady(ctx, config.AgentConfig.KubeConfigK3sController, util.DefaultAPIServerReadyTimeout, authorizationv1.ResourceAttributes{
			Namespace: metav1.NamespaceDefault,
			Verb:      "list",
			Resource:  "endpoints",
		}, ""); err != nil {
			logrus.Fatalf("Tunnel watches failed to wait for RBAC: %v", err)
		}

		close(apiServerReady)
	}()

	// We don't need to run the tunnel authorizer if the container runtime endpoint is /dev/null,
	// signifying that this is an agentless server that will not register a node.
	if config.ContainerRuntimeEndpoint != "/dev/null" {
		// Allow the kubelet port, as published via our node object.
		go tunnel.setKubeletPort(ctx, apiServerReady)

		switch tunnel.mode {
		case daemonconfig.EgressSelectorModeCluster:
			// In Cluster mode, we allow the cluster CIDRs, and any connections to the node's IPs for pods using host network.
			tunnel.clusterAuth(config)
		case daemonconfig.EgressSelectorModePod:
			// In Pod mode, we watch pods assigned to this node, and allow their addresses, as well as ports used by containers with host network.
			go tunnel.watchPods(ctx, apiServerReady, config)
		}
	}

	// The loadbalancer is only disabled when there is a local apiserver.  Servers without a local
	// apiserver load-balance to themselves initially, then switch over to an apiserver node as soon
	// as we get some addresses from the code below.
	var localSupervisorDefault bool
	if addresses := proxy.SupervisorAddresses(); len(addresses) > 0 {
		host, _, _ := net.SplitHostPort(addresses[0])
		if host == "127.0.0.1" || host == "::1" {
			localSupervisorDefault = true
		}
	}

	if proxy.IsSupervisorLBEnabled() && proxy.SupervisorURL() != "" {
		logrus.Info("Getting list of apiserver endpoints from server")
		// If not running an apiserver locally, try to get a list of apiservers from the server we're
		// connecting to. If that fails, fall back to querying the endpoints list from Kubernetes. This
		// fallback requires that the server we're joining be running an apiserver, but is the only safe
		// thing to do if its supervisor is down-level and can't provide us with an endpoint list.
		addresses := agentconfig.APIServers(ctx, config, proxy)
		logrus.Infof("Got apiserver addresses from supervisor: %v", addresses)

		if len(addresses) > 0 {
			if localSupervisorDefault {
				proxy.SetSupervisorDefault(addresses[0])
			}
			proxy.Update(addresses)
		} else {
			if endpoint, _ := client.CoreV1().Endpoints(metav1.NamespaceDefault).Get(ctx, "kubernetes", metav1.GetOptions{}); endpoint != nil {
				addresses = util.GetAddresses(endpoint)
				logrus.Infof("Got apiserver addresses from kubernetes endpoints: %v", addresses)
				if len(addresses) > 0 {
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

// setKubeletPort retrieves the configured kubelet port from our node object
func (a *agentTunnel) setKubeletPort(ctx context.Context, apiServerReady <-chan struct{}) {
	<-apiServerReady

	wait.PollUntilContextTimeout(ctx, time.Second, util.DefaultAPIServerReadyTimeout, true, func(ctx context.Context) (bool, error) {
		var readyTime metav1.Time
		nodeName := os.Getenv("NODE_NAME")
		node, err := a.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			logrus.Debugf("Tunnel authorizer failed to get Kubelet Port: %v", err)
			return false, nil
		}
		for _, cond := range node.Status.Conditions {
			if cond.Type == v1.NodeReady && cond.Status == v1.ConditionTrue {
				readyTime = cond.LastHeartbeatTime
			}
		}
		if readyTime.Time.Before(a.startTime) {
			logrus.Debugf("Waiting for Ready condition to be updated for Kubelet Port assignment")
			return false, nil
		}
		kubeletPort := strconv.FormatInt(int64(node.Status.DaemonEndpoints.KubeletEndpoint.Port), 10)
		if kubeletPort == "0" {
			logrus.Debugf("Waiting for Kubelet Port to be set")
			return false, nil
		}
		a.kubeletPort = kubeletPort
		logrus.Infof("Tunnel authorizer set Kubelet Port %s", net.JoinHostPort(a.kubeletAddr, a.kubeletPort))
		return true, nil
	})
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
			conn := a.connect(ctx, wg, address, tlsConfig)
			disconnect[address] = conn.cancel
			proxy.SetHealthCheck(address, conn.connected)
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

	var cancelUpdate context.CancelFunc

	for {
		select {
		case <-ctx.Done():
			if cancelUpdate != nil {
				cancelUpdate()
			}
			return
		case ev, ok := <-watch.ResultChan():
			endpoint, ok := ev.Object.(*v1.Endpoints)
			if !ok {
				logrus.Errorf("Tunnel watch failed: event object not of type v1.Endpoints")
				continue
			}

			if cancelUpdate != nil {
				cancelUpdate()
			}

			var debounceCtx context.Context
			debounceCtx, cancelUpdate = context.WithCancel(ctx)

			// When joining the cluster, the apiserver adds, removes, and then re-adds itself to
			// the endpoint list several times.  This causes a bit of thrashing if we react to
			// endpoint changes immediately.  Instead, perform the endpoint update in a
			// goroutine that sleeps for a short period before checking for changes and updating
			// the proxy addresses.  If another update occurs, the previous update operation
			// will be cancelled and a new one queued.
			go func() {
				select {
				case <-time.After(endpointDebounceDelay):
				case <-debounceCtx.Done():
					return
				}

				newAddresses := util.GetAddresses(endpoint)
				if reflect.DeepEqual(newAddresses, proxy.SupervisorAddresses()) {
					return
				}
				proxy.Update(newAddresses)

				validEndpoint := map[string]bool{}

				for _, address := range proxy.SupervisorAddresses() {
					validEndpoint[address] = true
					if _, ok := disconnect[address]; !ok {
						conn := a.connect(ctx, nil, address, tlsConfig)
						disconnect[address] = conn.cancel
						proxy.SetHealthCheck(address, conn.connected)
					}
				}

				for address, cancel := range disconnect {
					if !validEndpoint[address] {
						cancel()
						delete(disconnect, address)
						logrus.Infof("Stopped tunnel to %s", address)
					}
				}
			}()
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
		if a.isKubeletOrStreamPort(proto, host, port) {
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

type agentConnection struct {
	cancel    context.CancelFunc
	connected func() bool
}

// connect initiates a connection to the remotedialer server. Incoming dial requests from
// the server will be checked by the authorizer function prior to being fulfilled.
func (a *agentTunnel) connect(rootCtx context.Context, waitGroup *sync.WaitGroup, address string, tlsConfig *tls.Config) agentConnection {
	wsURL := fmt.Sprintf("wss://%s/v1-"+version.Program+"/connect", address)
	ws := &websocket.Dialer{
		TLSClientConfig: tlsConfig,
	}

	// Assume that the connection to the server will succeed, to avoid failing health checks while attempting to connect.
	// If we cannot connect, connected will be set to false when the initial connection attempt fails.
	connected := true

	once := sync.Once{}
	if waitGroup != nil {
		waitGroup.Add(1)
	}

	ctx, cancel := context.WithCancel(rootCtx)
	auth := func(proto, address string) bool {
		return a.authorized(rootCtx, proto, address)
	}

	onConnect := func(_ context.Context, _ *remotedialer.Session) error {
		connected = true
		logrus.WithField("url", wsURL).Info("Remotedialer connected to proxy")
		if waitGroup != nil {
			once.Do(waitGroup.Done)
		}
		return nil
	}

	// Start remotedialer connect loop in a goroutine to ensure a connection to the target server
	go func() {
		for {
			// ConnectToProxy blocks until error or context cancellation
			err := remotedialer.ConnectToProxyWithDialer(ctx, wsURL, nil, auth, ws, a.dialContext, onConnect)
			connected = false
			if err != nil && !errors.Is(err, context.Canceled) {
				logrus.WithField("url", wsURL).WithError(err).Error("Remotedialer proxy error; reconnecting...")
				// wait between reconnection attempts to avoid hammering the server
				time.Sleep(endpointDebounceDelay)
			}
			// If the context has been cancelled, exit the goroutine instead of retrying
			if ctx.Err() != nil {
				if waitGroup != nil {
					once.Do(waitGroup.Done)
				}
				return
			}
		}
	}()

	return agentConnection{
		cancel:    cancel,
		connected: func() bool { return connected },
	}
}

// isKubeletOrStreamPort returns true if the connection is to a reserved TCP port on a loopback address.
func (a *agentTunnel) isKubeletOrStreamPort(proto, host, port string) bool {
	return proto == "tcp" && (host == "127.0.0.1" || host == "::1") && (port == a.kubeletPort || port == daemonconfig.StreamServerPort)
}

// dialContext dials a local connection on behalf of the remote server.  If the
// connection is to the kubelet port on the loopback address, the kubelet is dialed
// at its configured bind address.  Otherwise, the connection is dialed normally.
func (a *agentTunnel) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if a.isKubeletOrStreamPort(network, host, port) && port == a.kubeletPort {
		address = net.JoinHostPort(a.kubeletAddr, port)
	}
	return defaultDialer.DialContext(ctx, network, address)
}
