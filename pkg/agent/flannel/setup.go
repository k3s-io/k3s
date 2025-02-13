package flannel

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	agentutil "github.com/k3s-io/k3s/pkg/agent/util"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	authorizationv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
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

	ipv4 = iota
	ipv6
)

func Prepare(ctx context.Context, nodeConfig *config.Node) error {
	if err := createCNIConf(nodeConfig.AgentConfig.CNIConfDir, nodeConfig); err != nil {
		return err
	}

	return createFlannelConf(nodeConfig)
}

func Run(ctx context.Context, nodeConfig *config.Node) error {
	logrus.Infof("Starting flannel with backend %s", nodeConfig.FlannelBackend)

	kubeConfig := nodeConfig.AgentConfig.KubeConfigKubelet
	resourceAttrs := authorizationv1.ResourceAttributes{Verb: "list", Resource: "nodes"}

	// Compatibility code for AuthorizeNodeWithSelectors feature-gate.
	// If the kubelet cannot list nodes, then wait for the k3s-controller RBAC to become ready, and use that kubeconfig instead.
	if canListNodes, err := util.CheckRBAC(ctx, kubeConfig, resourceAttrs, ""); err != nil {
		return errors.Wrap(err, "failed to check if RBAC allows node list")
	} else if !canListNodes {
		kubeConfig = nodeConfig.AgentConfig.KubeConfigK3sController
		if err := util.WaitForRBACReady(ctx, kubeConfig, util.DefaultAPIServerReadyTimeout, resourceAttrs, ""); err != nil {
			return errors.Wrap(err, "flannel failed to wait for RBAC")
		}
	}

	coreClient, err := util.GetClientSet(kubeConfig)
	if err != nil {
		return err
	}

	if err := waitForPodCIDR(ctx, nodeConfig.AgentConfig.NodeName, coreClient.CoreV1().Nodes()); err != nil {
		return errors.Wrap(err, "flannel failed to wait for PodCIDR assignment")
	}

	netMode, err := findNetMode(nodeConfig.AgentConfig.ClusterCIDRs)
	if err != nil {
		return errors.Wrap(err, "failed to check netMode for flannel")
	}
	go func() {
		err := flannel(ctx, nodeConfig.FlannelIface, nodeConfig.FlannelConfFile, kubeConfig, nodeConfig.FlannelIPv6Masq, netMode)
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Errorf("flannel exited: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	return nil
}

// waitForPodCIDR watches nodes with this node's name, and returns when the PodCIDR has been set.
func waitForPodCIDR(ctx context.Context, nodeName string, nodes typedcorev1.NodeInterface) error {
	fieldSelector := fields.Set{metav1.ObjectNameField: nodeName}.String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (object runtime.Object, e error) {
			options.FieldSelector = fieldSelector
			return nodes.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return nodes.Watch(ctx, options)
		},
	}
	condition := func(ev watch.Event) (bool, error) {
		if n, ok := ev.Object.(*v1.Node); ok {
			return n.Spec.PodCIDR != "", nil
		}
		return false, errors.New("event object not of type v1.Node")
	}

	if _, err := toolswatch.UntilWithSync(ctx, lw, &v1.Node{}, nil, condition); err != nil {
		return errors.Wrap(err, "failed to wait for PodCIDR assignment")
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

	if nodeConfig.AgentConfig.FlannelCniConfFile != "" {
		logrus.Debugf("Using %s as the flannel CNI conf", nodeConfig.AgentConfig.FlannelCniConfFile)
		return agentutil.CopyFile(nodeConfig.AgentConfig.FlannelCniConfFile, p, false)
	}

	cniConfJSON := cniConf
	if goruntime.GOOS == "windows" {
		extIface, err := LookupExtInterface(nodeConfig.FlannelIface, ipv4)
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
	var ipv4Enabled string
	logrus.Debugf("Creating the flannel configuration for backend %s in file %s", nodeConfig.FlannelBackend, nodeConfig.FlannelConfFile)
	if nodeConfig.FlannelConfFile == "" {
		return errors.New("Flannel configuration not defined")
	}
	if nodeConfig.FlannelConfOverride {
		logrus.Infof("Using custom flannel conf defined at %s", nodeConfig.FlannelConfFile)
		return nil
	}
	netMode, err := findNetMode(nodeConfig.AgentConfig.ClusterCIDRs)
	if err != nil {
		logrus.Fatalf("Flannel error checking netMode: %v", err)
		return err
	}
	if netMode == ipv4 || netMode == (ipv4+ipv6) {
		ipv4Enabled = "true"
	} else {
		ipv4Enabled = "false"
	}
	confJSON := strings.ReplaceAll(flannelConf, "%IPV4_ENABLED%", ipv4Enabled)
	if netMode == ipv4 {
		confJSON = strings.ReplaceAll(confJSON, "%CIDR%", nodeConfig.AgentConfig.ClusterCIDR.String())
		confJSON = strings.ReplaceAll(confJSON, "%IPV6_ENABLED%", "false")
		confJSON = strings.ReplaceAll(confJSON, "%CIDR_IPV6%", emptyIPv6Network)
	} else if netMode == (ipv4 + ipv6) {
		confJSON = strings.ReplaceAll(confJSON, "%IPV6_ENABLED%", "true")
		for _, cidr := range nodeConfig.AgentConfig.ClusterCIDRs {
			if utilsnet.IsIPv6(cidr.IP) {
				// Only one ipv6 range available. This might change in future: https://github.com/kubernetes/enhancements/issues/2593
				confJSON = strings.ReplaceAll(confJSON, "%CIDR_IPV6%", cidr.String())
			} else {
				confJSON = strings.ReplaceAll(confJSON, "%CIDR%", cidr.String())
			}
		}
	} else {
		confJSON = strings.ReplaceAll(confJSON, "%CIDR%", "0.0.0.0/0")
		confJSON = strings.ReplaceAll(confJSON, "%IPV6_ENABLED%", "true")
		for _, cidr := range nodeConfig.AgentConfig.ClusterCIDRs {
			if utilsnet.IsIPv6(cidr.IP) {
				// Only one ipv6 range available. This might change in future: https://github.com/kubernetes/enhancements/issues/2593
				confJSON = strings.ReplaceAll(confJSON, "%CIDR_IPV6%", cidr.String())
			}
		}
	}

	var backendConf string

	// precheck and error out unsupported flannel backends.
	switch nodeConfig.FlannelBackend {
	case config.FlannelBackendHostGW:
	case config.FlannelBackendTailscale:
	case config.FlannelBackendWireguardNative:
		if goruntime.GOOS == "windows" {
			return fmt.Errorf("unsupported flannel backend '%s' for Windows", nodeConfig.FlannelBackend)
		}
	}

	switch nodeConfig.FlannelBackend {
	case config.FlannelBackendVXLAN:
		backendConf = vxlanBackend
	case config.FlannelBackendHostGW:
		backendConf = hostGWBackend
	case config.FlannelBackendTailscale:
		var routes string
		switch netMode {
		case ipv4:
			routes = "$SUBNET"
		case (ipv4 + ipv6):
			routes = "$SUBNET,$IPV6SUBNET"
		case ipv6:
			routes = "$IPV6SUBNET"
		default:
			return fmt.Errorf("incorrect netMode for flannel tailscale backend")
		}
		backendConf = strings.ReplaceAll(tailscaledBackend, "%Routes%", routes)
	case config.FlannelBackendWireguardNative:
		backendConf = wireguardNativeBackend
	default:
		return fmt.Errorf("Cannot configure unknown flannel backend '%s'", nodeConfig.FlannelBackend)
	}
	confJSON = strings.ReplaceAll(confJSON, "%backend%", backendConf)

	logrus.Debugf("The flannel configuration is %s", confJSON)
	return agentutil.WriteFile(nodeConfig.FlannelConfFile, confJSON)
}

// fundNetMode returns the mode (ipv4, ipv6 or dual-stack) in which flannel is operating
func findNetMode(cidrs []*net.IPNet) (int, error) {
	dualStack, err := utilsnet.IsDualStackCIDRs(cidrs)
	if err != nil {
		return 0, err
	}
	if dualStack {
		return ipv4 + ipv6, nil
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
