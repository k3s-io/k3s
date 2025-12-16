package flannel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"

	agentutil "github.com/k3s-io/k3s/pkg/agent/util"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/signals"
	"github.com/k3s-io/k3s/pkg/vpn"
	"github.com/k3s-io/k3s/pkg/util"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	authorizationv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	toolscache "k8s.io/client-go/tools/cache"
	toolswatch "k8s.io/client-go/tools/watch"
	utilsnet "k8s.io/utils/net"
)

const (
	flannelConf = `{
	"Network": "%CIDR%",
	"EnableIPv6": %IPV6_ENABLED%,
	"EnableIPv4": %IPV4_ENABLED%,
	"IPv6Network": "%CIDR_IPV6%",
	"Backend": %backend%
}
`

	hostGWBackend = `{
	"Type": "host-gw"
}`

	tailscaledBackend = `{
	"Type": "extension",
	"PostStartupCommand": "tailscale set --accept-routes --advertise-routes=%Routes%",
	"ShutdownCommand": "tailscale down"
}`

	wireguardNativeBackend = `{
	"Type": "wireguard",
	"PersistentKeepaliveInterval": 25
}`

	emptyIPv6Network = "::/0"
)

type netMode int

func (nm netMode) IPv4Enabled() bool { return nm&ipv4 != 0 }
func (nm netMode) IPv6Enabled() bool { return nm&ipv6 != 0 }

const (
	ipv4 netMode = 1 << iota
	ipv6
)

func Prepare(ctx context.Context, nodeConfig *config.Node) error {
	if err := createCNIConf(nodeConfig.AgentConfig.CNIConfDir, nodeConfig); err != nil {
		return err
	}

	return createFlannelConf(nodeConfig)
}

func Run(ctx context.Context, wg *sync.WaitGroup, nodeConfig *config.Node) error {
	logrus.Infof("Starting flannel with backend %s", nodeConfig.Flannel.Backend)

	kubeConfig := nodeConfig.AgentConfig.KubeConfigKubelet
	coreClient, err := util.GetClientSet(kubeConfig)
	if err != nil {
		return err
	}

	// use the kubelet kubeconfig to add node annotations, as the k3s-controller
	// rbac does not allow create or update of nodes.
	if err := setAnnotations(ctx, nodeConfig, coreClient); err != nil {
		return pkgerrors.WithMessage(err, "flannel failed to set address annotations")
	}

	resourceAttrs := authorizationv1.ResourceAttributes{Verb: "list", Resource: "nodes"}

	// Compatibility code for AuthorizeNodeWithSelectors feature-gate.
	// Flannel needs to watch all nodes in the cluster, which the kubelet is not allowed to do on recent versions of Kubernetes.
	// If the kubelet cannot list nodes, then wait for the k3s-controller RBAC to become ready, and use that kubeconfig instead.
	if canListNodes, err := util.CheckRBAC(ctx, kubeConfig, resourceAttrs, ""); err != nil {
		return pkgerrors.WithMessage(err, "failed to check if RBAC allows node list")
	} else if !canListNodes {
		kubeConfig = nodeConfig.AgentConfig.KubeConfigK3sController
		coreClient, err = util.GetClientSet(kubeConfig)
		if err != nil {
			return err
		}
		if err := util.WaitForRBACReady(ctx, kubeConfig, util.DefaultAPIServerReadyTimeout, resourceAttrs, ""); err != nil {
			return pkgerrors.WithMessage(err, "flannel failed to wait for RBAC")
		}
	}

	if err := waitForPodCIDR(ctx, nodeConfig.AgentConfig.NodeName, coreClient); err != nil {
		return pkgerrors.WithMessage(err, "flannel failed to wait for PodCIDR assignment")
	}

	nm, err := findNetMode(nodeConfig.AgentConfig.ClusterCIDRs)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to check netMode for flannel")
	}
	go func() {
		err := flannel(ctx, wg, nodeConfig.Flannel.Iface, nodeConfig.Flannel.ConfFile, kubeConfig, nodeConfig.Flannel.IPv6Masq, nm)
		if err != nil && !errors.Is(err, context.Canceled) {
			signals.RequestShutdown(pkgerrors.WithMessage(err, "flannel exited"))
		}
		signals.RequestShutdown(nil)
	}()

	return nil
}

// waitForPodCIDR watches nodes with this node's name, and returns when the PodCIDR has been set.
func waitForPodCIDR(ctx context.Context, nodeName string, coreClient kubernetes.Interface) error {
	lw := toolscache.NewListWatchFromClient(coreClient.CoreV1().RESTClient(), "nodes", metav1.NamespaceNone, fields.OneTermEqualSelector(metav1.ObjectNameField, nodeName))
	condition := func(ev watch.Event) (bool, error) {
		if n, ok := ev.Object.(*v1.Node); ok {
			return n.Spec.PodCIDR != "", nil
		}
		return false, errors.New("event object not of type v1.Node")
	}

	if _, err := toolswatch.UntilWithSync(ctx, lw, &v1.Node{}, nil, condition); err != nil {
		return pkgerrors.WithMessage(err, "failed to wait for PodCIDR assignment")
	}

	logrus.Info("Flannel found PodCIDR assigned for node " + nodeName)
	return nil
}

func createCNIConf(dir string, nodeConfig *config.Node) error {
	logrus.Debugf("Creating the CNI conf in directory %s", dir)
	if dir == "" {
		return nil
	}
	p := filepath.Join(dir, "10-flannel.conflist")

	if nodeConfig.Flannel.CNIConfFile != "" {
		logrus.Debugf("Using %s as the flannel CNI conf", nodeConfig.Flannel.CNIConfFile)
		return agentutil.CopyFile(nodeConfig.Flannel.CNIConfFile, p, false)
	}

	cniConfJSON := cniConf
	if goruntime.GOOS == "windows" {
		extIface, err := LookupExtInterface(nodeConfig.Flannel.Iface, ipv4)
		if err != nil {
			return err
		}

		cniConfJSON = strings.ReplaceAll(cniConfJSON, "%IPV4_ADDRESS%", extIface.IfaceAddr.String())
		cniConfJSON = strings.ReplaceAll(cniConfJSON, "%CLUSTER_CIDR%", nodeConfig.AgentConfig.ClusterCIDR.String())
		cniConfJSON = strings.ReplaceAll(cniConfJSON, "%SERVICE_CIDR%", nodeConfig.AgentConfig.ServiceCIDR.String())
	}

	return agentutil.WriteFile(p, cniConfJSON)
}

func createFlannelConf(nodeConfig *config.Node) error {
	logrus.Debugf("Creating the flannel configuration for backend %s in file %s", nodeConfig.Flannel.Backend, nodeConfig.Flannel.ConfFile)
	if nodeConfig.Flannel.ConfFile == "" {
		return errors.New("Flannel configuration not defined")
	}
	if nodeConfig.Flannel.ConfOverride {
		logrus.Infof("Using custom flannel conf defined at %s", nodeConfig.Flannel.ConfFile)
		return nil
	}
	nm, err := findNetMode(nodeConfig.AgentConfig.ClusterCIDRs)
	if err != nil {
		logrus.Fatalf("Flannel error checking netMode: %v", err)
		return err
	}
	confJSON := flannelConf
	if nm.IPv4Enabled() {
		confJSON = strings.ReplaceAll(confJSON, "%IPV4_ENABLED%", "true")
		if nm.IPv6Enabled() {
			for _, cidr := range nodeConfig.AgentConfig.ClusterCIDRs {
				if utilsnet.IsIPv4(cidr.IP) {
					confJSON = strings.ReplaceAll(confJSON, "%CIDR%", cidr.String())
					break
				}
			}
		} else {
			confJSON = strings.ReplaceAll(confJSON, "%CIDR%", nodeConfig.AgentConfig.ClusterCIDR.String())
		}
	} else {
		confJSON = strings.ReplaceAll(confJSON, "%IPV4_ENABLED%", "false")
		confJSON = strings.ReplaceAll(confJSON, "%CIDR%", "0.0.0.0/0")
	}
	if nm.IPv6Enabled() {
		confJSON = strings.ReplaceAll(confJSON, "%IPV6_ENABLED%", "true")
		for _, cidr := range nodeConfig.AgentConfig.ClusterCIDRs {
			if utilsnet.IsIPv6(cidr.IP) {
				// Only one ipv6 range available. This might change in future: https://github.com/kubernetes/enhancements/issues/2593
				confJSON = strings.ReplaceAll(confJSON, "%CIDR_IPV6%", cidr.String())
				break
			}
		}
	} else {
		confJSON = strings.ReplaceAll(confJSON, "%IPV6_ENABLED%", "false")
		confJSON = strings.ReplaceAll(confJSON, "%CIDR_IPV6%", emptyIPv6Network)
	}

	// precheck and error out unsupported flannel backends for windows.
	if goruntime.GOOS == "windows" {
		switch nodeConfig.Flannel.Backend {
		case BackendVXLAN, BackendNone:
			// these are the only supported backends
		default:
			return fmt.Errorf("unsupported flannel backend '%s' for Windows", nodeConfig.Flannel.Backend)
		}
	}

	var backendConf string

	switch nodeConfig.Flannel.Backend {
	case BackendVXLAN:
		backendConf = vxlanBackend
	case BackendHostGW:
		backendConf = hostGWBackend
	case BackendTailscale:
		var routes []string
		if nm.IPv4Enabled() {
			routes = append(routes, "$SUBNET")
		}
		if nm.IPv6Enabled() {
			routes = append(routes, "$IPV6SUBNET")
		}
		if len(routes) == 0 {
			return errors.New("incorrect netMode for flannel tailscale backend")
		}
		advertisedRoutes, err := vpn.GetAdvertisedRoutes()
		if err == nil && advertisedRoutes != nil {
			for _, advertisedRoute := range advertisedRoutes {
				routes = append(routes, advertisedRoute.String())
			}
		}
		backendConf = strings.ReplaceAll(tailscaledBackend, "%Routes%", strings.Join(routes, ","))
	case BackendWireguardNative:
		backendConf = wireguardNativeBackend
	default:
		return fmt.Errorf("Cannot configure unknown flannel backend '%s'", nodeConfig.Flannel.Backend)
	}
	confJSON = strings.ReplaceAll(confJSON, "%backend%", backendConf)

	logrus.Debugf("The flannel configuration is %s", confJSON)
	return agentutil.WriteFile(nodeConfig.Flannel.ConfFile, confJSON)
}

// fundNetMode returns the mode (ipv4, ipv6 or dual-stack) in which flannel is operating
func findNetMode(cidrs []*net.IPNet) (netMode, error) {
	dualStack, err := utilsnet.IsDualStackCIDRs(cidrs)
	if err != nil {
		return 0, err
	}
	if dualStack {
		return ipv4 | ipv6, nil
	}

	for _, cidr := range cidrs {
		if utilsnet.IsIPv4CIDR(cidr) {
			return ipv4, nil
		}
		if utilsnet.IsIPv6CIDR(cidr) {
			return ipv6, nil
		}
	}
	return 0, errors.New("Failed checking netMode")
}

func setAnnotations(ctx context.Context, nodeConfig *config.Node, coreClient kubernetes.Interface) error {
	patch := util.NewPatchList()
	patcher := util.NewPatcher[*v1.Node](coreClient.CoreV1().Nodes())
	if nodeConfig.AgentConfig.NodeExternalIP != "" && nodeConfig.Flannel.ExternalIP {
		for _, ipAddress := range nodeConfig.AgentConfig.NodeExternalIPs {
			if utilsnet.IsIPv4(ipAddress) {
				patch.Add(ipAddress.String(), "metadata", "annotations", ExternalIPv4Annotation)
			}
			if utilsnet.IsIPv6(ipAddress) {
				patch.Add(ipAddress.String(), "metadata", "annotations", ExternalIPv6Annotation)
			}
		}
	}
	_, err := patcher.Patch(ctx, patch, nodeConfig.AgentConfig.NodeName)
	return err
}
