package control

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/proxy"
	"github.com/k3s-io/k3s/pkg/nodeconfig"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/remotedialer"
	"github.com/sirupsen/logrus"
	"github.com/yl2chen/cidranger"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes"
)

func loggingErrorWriter(rw http.ResponseWriter, req *http.Request, code int, err error) {
	logrus.Debugf("Tunnel server error: %d %v", code, err)
	rw.WriteHeader(code)
	rw.Write([]byte(err.Error()))
}

func setupTunnel(ctx context.Context, cfg *config.Control) (http.Handler, error) {
	tunnel := &TunnelServer{
		cidrs:  cidranger.NewPCTrieRanger(),
		config: cfg,
		server: remotedialer.New(authorizer, loggingErrorWriter),
		egress: map[string]bool{},
	}
	go tunnel.watch(ctx)
	return tunnel, nil
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

// explicit interface check
var _ http.Handler = &TunnelServer{}

type TunnelServer struct {
	sync.Mutex
	cidrs  cidranger.Ranger
	client kubernetes.Interface
	config *config.Control
	server *remotedialer.Server
	egress map[string]bool
}

// explicit interface check
var _ cidranger.RangerEntry = &tunnelEntry{}

type tunnelEntry struct {
	cidr    net.IPNet
	node    string
	kubelet bool
}

func (n *tunnelEntry) Network() net.IPNet {
	return n.cidr
}

// ServeHTTP handles either CONNECT requests, or websocket requests to the remotedialer server
func (t *TunnelServer) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	logrus.Debugf("Tunnel server handing %s %s request for %s from %s", req.Proto, req.Method, req.URL, req.RemoteAddr)
	if req.Method == http.MethodConnect {
		t.serveConnect(resp, req)
	} else {
		t.server.ServeHTTP(resp, req)
	}
}

// watch waits for the runtime core to become available,
// and registers OnChange handlers to observe changes to Nodes (and Endpoints if necessary).
func (t *TunnelServer) watch(ctx context.Context) {
	logrus.Infof("Tunnel server egress proxy mode: %s", t.config.EgressSelectorMode)

	if t.config.EgressSelectorMode == config.EgressSelectorModeDisabled {
		return
	}

	for {
		if t.config.Runtime.Core != nil {
			t.config.Runtime.Core.Core().V1().Node().OnChange(ctx, version.Program+"-tunnel-server", t.onChangeNode)
			if t.config.EgressSelectorMode == config.EgressSelectorModeCluster {
				// Cluster mode watches Endpoints to find what Node is hosting an Endpoint address, as the CNI
				// may be using its own IPAM that does not repsect the Node's PodCIDR.
				t.config.Runtime.Core.Core().V1().Endpoints().OnChange(ctx, version.Program+"-tunnel-server", t.onChangeEndpoints)
			}
			return
		}
		logrus.Infof("Tunnel server egress proxy waiting for runtime core to become available")
		time.Sleep(5 * time.Second)
	}
}

// onChangeNode updates the node address/CIDR mappings by observing changes to nodes.
// Node addresses are updated in Agent, Cluster, and Pod mode.
// Pod CIDRs are updated only in Pod mode
func (t *TunnelServer) onChangeNode(nodeName string, node *v1.Node) (*v1.Node, error) {
	if node != nil {
		t.Lock()
		defer t.Unlock()
		logrus.Debugf("Tunnel server egress proxy updating node %s", nodeName)
		_, t.egress[nodeName] = node.Labels[nodeconfig.ClusterEgressLabel]
		// Add all node IP addresses
		for _, addr := range node.Status.Addresses {
			if addr.Type == v1.NodeInternalIP || addr.Type == v1.NodeExternalIP {
				address := addr.Address
				if strings.Contains(address, ":") {
					address += "/128"
				} else {
					address += "/32"
				}
				if _, n, err := net.ParseCIDR(address); err == nil {
					if node.DeletionTimestamp != nil {
						t.cidrs.Remove(*n)
					} else {
						t.cidrs.Insert(&tunnelEntry{cidr: *n, node: nodeName, kubelet: true})
					}
				}
			}
		}
		// Add all Node PodCIDRs, if in pod mode
		if t.config.EgressSelectorMode == config.EgressSelectorModePod {
			for _, cidr := range node.Spec.PodCIDRs {
				if _, n, err := net.ParseCIDR(cidr); err == nil {
					if node.DeletionTimestamp != nil {
						t.cidrs.Remove(*n)
					} else {
						t.cidrs.Insert(&tunnelEntry{cidr: *n, node: nodeName})
					}
				}
			}
		}
	}
	return node, nil
}

// onChangeEndpoits updates the pod address mappings by observing changes to endpoints.
// Only Pod endpoints with a defined NodeName are used, and only in Cluster mode.
func (t *TunnelServer) onChangeEndpoints(endpointsName string, endpoints *v1.Endpoints) (*v1.Endpoints, error) {
	if endpoints != nil {
		t.Lock()
		defer t.Unlock()
		logrus.Debugf("Tunnel server egress proxy updating endpoints %s", endpointsName)
		// Add all Pod endpoints
		for _, subset := range endpoints.Subsets {
			for _, addr := range subset.Addresses {
				if addr.NodeName != nil && addr.TargetRef != nil && addr.TargetRef.Kind == "Pod" {
					nodeName := *addr.NodeName
					address := addr.IP
					if strings.Contains(address, ":") {
						address += "/128"
					} else {
						address += "/32"
					}
					if _, n, err := net.ParseCIDR(address); err == nil {
						t.cidrs.Insert(&tunnelEntry{cidr: *n, node: nodeName})
					}
				}
			}
			for _, addr := range subset.NotReadyAddresses {
				if addr.TargetRef != nil && addr.TargetRef.Kind == "Pod" {
					address := addr.IP
					if strings.Contains(address, ":") {
						address += "/128"
					} else {
						address += "/32"
					}
					if _, n, err := net.ParseCIDR(address); err == nil {
						t.cidrs.Remove(*n)
					}
				}
			}
		}
	}
	return endpoints, nil
}

// serveConnect attempts to handle the HTTP CONNECT request by dialing
// a connection, either locally or via the remotedialer tunnel.
func (t *TunnelServer) serveConnect(resp http.ResponseWriter, req *http.Request) {
	bconn, err := t.dialBackend(req.Host)
	if err != nil {
		http.Error(resp, fmt.Sprintf("no tunnels available: %v", err), http.StatusInternalServerError)
		return
	}

	hijacker, ok := resp.(http.Hijacker)
	if !ok {
		http.Error(resp, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	resp.WriteHeader(http.StatusOK)

	rconn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(resp, err.Error(), http.StatusInternalServerError)
		return
	}

	proxy.Proxy(rconn, bconn)
}

// dialBackend determines where to route the connection request to, and returns
// a dialed connection if possible. Note that in the case of a remotedialer
// tunnel connection, the agent may return an error if the agent's authorizer
// denies the connection, or if there is some other error in actually dialing
// the requested endpoint.
func (t *TunnelServer) dialBackend(addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	loopback := t.config.Loopback()

	var node string
	var toKubelet, useTunnel bool
	if ip := net.ParseIP(host); ip != nil {
		// Destination is an IP address, check to see if the target is a kubelet or pod address.
		// We can only use the tunnel for egress to pods if the agent supports it.
		if nets, err := t.cidrs.ContainingNetworks(ip); err == nil && len(nets) > 0 {
			if n, ok := nets[0].(*tunnelEntry); ok {
				node = n.node
				if n.kubelet {
					toKubelet = true
					useTunnel = true
				} else {
					useTunnel = t.egress[node]
				}
			} else {
				logrus.Debugf("Tunnel server egress proxy CIDR lookup returned unknown type for address %s", ip)
			}
		}
	} else {
		// Destination is a kubelet by name, it is safe to use the tunnel.
		node = host
		toKubelet = true
		useTunnel = true
	}

	// Always dial kubelets via the loopback address.
	if toKubelet {
		addr = fmt.Sprintf("%s:%s", loopback, port)
	}

	// If connecting to something hosted by the local node, don't tunnel
	if node == t.config.ServerNodeName {
		useTunnel = false
	}

	if t.server.HasSession(node) {
		if useTunnel {
			// Have a session and it is safe to use for this destination, do so.
			logrus.Debugf("Tunnel server egress proxy dialing %s via session to %s", addr, node)
			return t.server.Dial(node, 15*time.Second, "tcp", addr)
		}
		// Have a session but the agent doesn't support tunneling to this destination or
		// the destination is local; fall back to direct connection.
		logrus.Debugf("Tunnel server egress proxy dialing %s directly", addr)
		return net.Dial("tcp", addr)
	}

	// don't provide a proxy connection for anything else
	logrus.Debugf("Tunnel server egress proxy rejecting connection to %s", addr)
	return nil, fmt.Errorf("no sessions available for host %s", host)
}
