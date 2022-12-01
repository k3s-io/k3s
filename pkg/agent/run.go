package agent

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	systemd "github.com/coreos/go-systemd/daemon"
	"github.com/k3s-io/k3s/pkg/agent/config"
	"github.com/k3s-io/k3s/pkg/agent/containerd"
	"github.com/k3s-io/k3s/pkg/agent/cridockerd"
	"github.com/k3s-io/k3s/pkg/agent/flannel"
	"github.com/k3s-io/k3s/pkg/agent/netpol"
	"github.com/k3s-io/k3s/pkg/agent/proxy"
	"github.com/k3s-io/k3s/pkg/agent/syssetup"
	"github.com/k3s-io/k3s/pkg/agent/tunnel"
	"github.com/k3s-io/k3s/pkg/cgroups"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	cp "github.com/k3s-io/k3s/pkg/cloudprovider"
	"github.com/k3s-io/k3s/pkg/daemons/agent"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	types "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/nodeconfig"
	"github.com/k3s-io/k3s/pkg/rootless"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	toolswatch "k8s.io/client-go/tools/watch"
	app2 "k8s.io/kubernetes/cmd/kube-proxy/app"
	kubeproxyconfig "k8s.io/kubernetes/pkg/proxy/apis/config"
	utilsnet "k8s.io/utils/net"
	utilpointer "k8s.io/utils/pointer"
)

func run(ctx context.Context, cfg cmds.Agent, proxy proxy.Proxy) error {
	nodeConfig := config.Get(ctx, cfg, proxy)

	dualCluster, err := utilsnet.IsDualStackCIDRs(nodeConfig.AgentConfig.ClusterCIDRs)
	if err != nil {
		return errors.Wrap(err, "failed to validate cluster-cidr")
	}
	dualService, err := utilsnet.IsDualStackCIDRs(nodeConfig.AgentConfig.ServiceCIDRs)
	if err != nil {
		return errors.Wrap(err, "failed to validate service-cidr")
	}
	dualNode, err := utilsnet.IsDualStackIPs(nodeConfig.AgentConfig.NodeIPs)
	if err != nil {
		return errors.Wrap(err, "failed to validate node-ip")
	}
	serviceIPv4 := utilsnet.IsIPv4CIDR(nodeConfig.AgentConfig.ServiceCIDR)
	clusterIPv4 := utilsnet.IsIPv4CIDR(nodeConfig.AgentConfig.ClusterCIDR)
	nodeIPv4 := utilsnet.IsIPv4String(nodeConfig.AgentConfig.NodeIP)
	serviceIPv6 := utilsnet.IsIPv6CIDR(nodeConfig.AgentConfig.ServiceCIDR)
	clusterIPv6 := utilsnet.IsIPv6CIDR(nodeConfig.AgentConfig.ClusterCIDR)
	nodeIPv6 := utilsnet.IsIPv6String(nodeConfig.AgentConfig.NodeIP)
	if (serviceIPv6 != clusterIPv6) || (dualCluster != dualService) || (serviceIPv4 != clusterIPv4) {
		return fmt.Errorf("cluster-cidr: %v and service-cidr: %v, must share the same IP version (IPv4, IPv6 or dual-stack)", nodeConfig.AgentConfig.ClusterCIDRs, nodeConfig.AgentConfig.ServiceCIDRs)
	}
	if (clusterIPv6 && !nodeIPv6) || (dualCluster && !dualNode) || (clusterIPv4 && !nodeIPv4) {
		return fmt.Errorf("cluster-cidr: %v and node-ip: %v, must share the same IP version (IPv4, IPv6 or dual-stack)", nodeConfig.AgentConfig.ClusterCIDRs, nodeConfig.AgentConfig.NodeIPs)
	}
	enableIPv6 := dualCluster || clusterIPv6
	enableIPv4 := dualCluster || clusterIPv4

	conntrackConfig, err := getConntrackConfig(nodeConfig)
	if err != nil {
		return errors.Wrap(err, "failed to validate kube-proxy conntrack configuration")
	}
	syssetup.Configure(enableIPv6, conntrackConfig)
	nodeConfig.AgentConfig.EnableIPv4 = enableIPv4
	nodeConfig.AgentConfig.EnableIPv6 = enableIPv6

	if err := setupCriCtlConfig(cfg, nodeConfig); err != nil {
		return err
	}

	if err := executor.Bootstrap(ctx, nodeConfig, cfg); err != nil {
		return err
	}

	if !nodeConfig.NoFlannel {
		if (nodeConfig.FlannelExternalIP) && (len(nodeConfig.AgentConfig.NodeExternalIPs) == 0) {
			logrus.Warnf("Server has flannel-external-ip flag set but this node does not set node-external-ip. Flannel will use internal address when connecting to this node.")
		} else if (nodeConfig.FlannelExternalIP) && (nodeConfig.FlannelBackend != types.FlannelBackendWireguardNative) && (nodeConfig.FlannelBackend != types.FlannelBackendIPSEC) {
			logrus.Warnf("Flannel is using external addresses with an insecure backend: %v. Please consider using an encrypting flannel backend.", nodeConfig.FlannelBackend)
		}
		if err := flannel.Prepare(ctx, nodeConfig); err != nil {
			return err
		}
	}

	if nodeConfig.Docker {
		if err := cridockerd.Run(ctx, nodeConfig); err != nil {
			return err
		}
	} else if nodeConfig.ContainerRuntimeEndpoint == "" {
		if err := containerd.Run(ctx, nodeConfig); err != nil {
			return err
		}
	}

	// the agent runtime is ready to host workloads when containerd is up and the airgap
	// images have finished loading, as that portion of startup may block for an arbitrary
	// amount of time depending on how long it takes to import whatever the user has placed
	// in the images directory.
	if cfg.AgentReady != nil {
		close(cfg.AgentReady)
	}

	notifySocket := os.Getenv("NOTIFY_SOCKET")
	os.Unsetenv("NOTIFY_SOCKET")

	if err := setupTunnelAndRunAgent(ctx, nodeConfig, cfg, proxy); err != nil {
		return err
	}

	if err := util.WaitForAPIServerReady(ctx, nodeConfig.AgentConfig.KubeConfigKubelet, util.DefaultAPIServerReadyTimeout); err != nil {
		return errors.Wrap(err, "failed to wait for apiserver ready")
	}

	coreClient, err := coreClient(nodeConfig.AgentConfig.KubeConfigKubelet)
	if err != nil {
		return err
	}

	if err := configureNode(ctx, nodeConfig, coreClient.CoreV1().Nodes()); err != nil {
		return err
	}

	if !nodeConfig.NoFlannel {
		if err := flannel.Run(ctx, nodeConfig, coreClient.CoreV1().Nodes()); err != nil {
			return err
		}
	}

	if !nodeConfig.AgentConfig.DisableNPC {
		if err := netpol.Run(ctx, nodeConfig); err != nil {
			return err
		}
	}

	// By default, the server is responsible for notifying systemd
	// On agent-only nodes, the agent will notify systemd
	if notifySocket != "" {
		logrus.Info(version.Program + " agent is up and running")
		os.Setenv("NOTIFY_SOCKET", notifySocket)
		systemd.SdNotify(true, "READY=1\n")
	}

	<-ctx.Done()
	return ctx.Err()
}

// getConntrackConfig uses the kube-proxy code to parse the user-provided kube-proxy-arg values, and
// extract the conntrack settings so that K3s can set them itself. This allows us to soft-fail when
// running K3s in Docker, where kube-proxy is no longer allowed to set conntrack sysctls on newer kernels.
// When running rootless, we do not attempt to set conntrack sysctls - this behavior is copied from kubeadm.
func getConntrackConfig(nodeConfig *daemonconfig.Node) (*kubeproxyconfig.KubeProxyConntrackConfiguration, error) {
	ctConfig := &kubeproxyconfig.KubeProxyConntrackConfiguration{
		MaxPerCore:            utilpointer.Int32Ptr(0),
		Min:                   utilpointer.Int32Ptr(0),
		TCPEstablishedTimeout: &metav1.Duration{},
		TCPCloseWaitTimeout:   &metav1.Duration{},
	}

	if nodeConfig.AgentConfig.Rootless {
		return ctConfig, nil
	}

	cmd := app2.NewProxyCommand()
	if err := cmd.ParseFlags(daemonconfig.GetArgs(map[string]string{}, nodeConfig.AgentConfig.ExtraKubeProxyArgs)); err != nil {
		return nil, err
	}
	maxPerCore, err := cmd.Flags().GetInt32("conntrack-max-per-core")
	if err != nil {
		return nil, err
	}
	ctConfig.MaxPerCore = &maxPerCore
	min, err := cmd.Flags().GetInt32("conntrack-min")
	if err != nil {
		return nil, err
	}
	ctConfig.Min = &min
	establishedTimeout, err := cmd.Flags().GetDuration("conntrack-tcp-timeout-established")
	if err != nil {
		return nil, err
	}
	ctConfig.TCPEstablishedTimeout.Duration = establishedTimeout
	closeWaitTimeout, err := cmd.Flags().GetDuration("conntrack-tcp-timeout-close-wait")
	if err != nil {
		return nil, err
	}
	ctConfig.TCPCloseWaitTimeout.Duration = closeWaitTimeout
	return ctConfig, nil
}

func coreClient(cfg string) (kubernetes.Interface, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", cfg)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(restConfig)
}

// RunStandalone bootstraps the executor, but does not run the kubelet or containerd.
// This allows other bits of code that expect the executor to be set up properly to function
// even when the agent is disabled. It will only return in case of error or context
// cancellation.
func RunStandalone(ctx context.Context, cfg cmds.Agent) error {
	proxy, err := createProxyAndValidateToken(ctx, &cfg)
	if err != nil {
		return err
	}

	nodeConfig := config.Get(ctx, cfg, proxy)
	if err := executor.Bootstrap(ctx, nodeConfig, cfg); err != nil {
		return err
	}

	if cfg.AgentReady != nil {
		close(cfg.AgentReady)
	}

	if err := tunnelSetup(ctx, nodeConfig, cfg, proxy); err != nil {
		return err
	}

	<-ctx.Done()
	return ctx.Err()
}

// Run sets up cgroups, configures the LB proxy, and triggers startup
// of containerd and kubelet. It will only return in case of error or context
// cancellation.
func Run(ctx context.Context, cfg cmds.Agent) error {
	if err := cgroups.Validate(); err != nil {
		return err
	}

	if cfg.Rootless && !cfg.RootlessAlreadyUnshared {
		dualNode, err := utilsnet.IsDualStackIPStrings(cfg.NodeIP)
		if err != nil {
			return err
		}
		if err := rootless.Rootless(cfg.DataDir, dualNode); err != nil {
			return err
		}
	}

	proxy, err := createProxyAndValidateToken(ctx, &cfg)
	if err != nil {
		return err
	}

	return run(ctx, cfg, proxy)
}

func createProxyAndValidateToken(ctx context.Context, cfg *cmds.Agent) (proxy.Proxy, error) {
	agentDir := filepath.Join(cfg.DataDir, "agent")
	if err := os.MkdirAll(agentDir, 0700); err != nil {
		return nil, err
	}
	_, isIPv6, _ := util.GetFirstString([]string{cfg.NodeIP.String()})

	proxy, err := proxy.NewSupervisorProxy(ctx, !cfg.DisableLoadBalancer, agentDir, cfg.ServerURL, cfg.LBServerPort, isIPv6)
	if err != nil {
		return nil, err
	}

	for {
		newToken, err := clientaccess.ParseAndValidateTokenForUser(proxy.SupervisorURL(), cfg.Token, "node")
		if err != nil {
			logrus.Error(err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}
		cfg.Token = newToken.String()
		break
	}
	return proxy, nil
}

// configureNode waits for the node object to be created, and if/when it does,
// ensures that the labels and annotations are up to date.
func configureNode(ctx context.Context, nodeConfig *daemonconfig.Node, nodes typedcorev1.NodeInterface) error {
	agentConfig := &nodeConfig.AgentConfig
	fieldSelector := fields.Set{metav1.ObjectNameField: agentConfig.NodeName}.String()
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
		node, ok := ev.Object.(*v1.Node)
		if !ok {
			return false, errors.New("event object not of type v1.Node")
		}

		updateNode := false
		if labels, changed := updateMutableLabels(agentConfig, node.Labels); changed {
			node.Labels = labels
			updateNode = true
		}

		if !agentConfig.DisableCCM {
			if annotations, changed := updateAddressAnnotations(nodeConfig, node.Annotations); changed {
				node.Annotations = annotations
				updateNode = true
			}
			if labels, changed := updateLegacyAddressLabels(agentConfig, node.Labels); changed {
				node.Labels = labels
				updateNode = true
			}
		}

		// inject node config
		if changed, err := nodeconfig.SetNodeConfigAnnotations(node); err != nil {
			return false, err
		} else if changed {
			updateNode = true
		}

		if changed, err := nodeconfig.SetNodeConfigLabels(node); err != nil {
			return false, err
		} else if changed {
			updateNode = true
		}

		if updateNode {
			if _, err := nodes.Update(ctx, node, metav1.UpdateOptions{}); err != nil {
				logrus.Infof("Failed to set annotations and labels on node %s: %v", agentConfig.NodeName, err)
				return false, nil
			}
			logrus.Infof("Annotations and labels have been set successfully on node: %s", agentConfig.NodeName)
			return true, nil
		}
		logrus.Infof("Annotations and labels have already set on node: %s", agentConfig.NodeName)
		return true, nil
	}

	if _, err := toolswatch.UntilWithSync(ctx, lw, &v1.Node{}, nil, condition); err != nil {
		return errors.Wrap(err, "failed to configure node")
	}
	return nil
}

func updateMutableLabels(agentConfig *daemonconfig.Agent, nodeLabels map[string]string) (map[string]string, bool) {
	result := map[string]string{}

	for _, m := range agentConfig.NodeLabels {
		var (
			v string
			p = strings.SplitN(m, `=`, 2)
			k = p[0]
		)
		if len(p) > 1 {
			v = p[1]
		}
		result[k] = v
	}
	result = labels.Merge(nodeLabels, result)
	return result, !equality.Semantic.DeepEqual(nodeLabels, result)
}

func updateLegacyAddressLabels(agentConfig *daemonconfig.Agent, nodeLabels map[string]string) (map[string]string, bool) {
	ls := labels.Set(nodeLabels)
	if ls.Has(cp.InternalIPKey) || ls.Has(cp.HostnameKey) {
		result := map[string]string{
			cp.InternalIPKey: agentConfig.NodeIP,
			cp.HostnameKey:   agentConfig.NodeName,
		}

		if agentConfig.NodeExternalIP != "" {
			result[cp.ExternalIPKey] = agentConfig.NodeExternalIP
		}

		result = labels.Merge(nodeLabels, result)
		return result, !equality.Semantic.DeepEqual(nodeLabels, result)
	}
	return nil, false
}

// updateAddressAnnotations updates the node annotations with important information about IP addresses of the node
func updateAddressAnnotations(nodeConfig *daemonconfig.Node, nodeAnnotations map[string]string) (map[string]string, bool) {
	agentConfig := &nodeConfig.AgentConfig
	result := map[string]string{
		cp.InternalIPKey: util.JoinIPs(agentConfig.NodeIPs),
		cp.HostnameKey:   agentConfig.NodeName,
	}

	if agentConfig.NodeExternalIP != "" {
		result[cp.ExternalIPKey] = util.JoinIPs(agentConfig.NodeExternalIPs)
		if nodeConfig.FlannelExternalIP {
			for _, ipAddress := range agentConfig.NodeExternalIPs {
				if utilsnet.IsIPv4(ipAddress) {
					result[flannel.FlannelExternalIPv4Annotation] = ipAddress.String()
				}
				if utilsnet.IsIPv6(ipAddress) {
					result[flannel.FlannelExternalIPv6Annotation] = ipAddress.String()
				}
			}
		}
	}

	result = labels.Merge(nodeAnnotations, result)
	return result, !equality.Semantic.DeepEqual(nodeAnnotations, result)
}

// setupTunnelAndRunAgent should start the setup tunnel before starting kubelet and kubeproxy
// there are special case for etcd agents, it will wait until it can find the apiaddress from
// the address channel and update the proxy with the servers addresses, if in rke2 we need to
// start the agent before the tunnel is setup to allow kubelet to start first and start the pods
func setupTunnelAndRunAgent(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent, proxy proxy.Proxy) error {
	var agentRan bool
	// IsAPIServerLBEnabled is used as a shortcut for detecting RKE2, where the kubelet needs to
	// be run earlier in order to manage static pods. This should probably instead query a
	// flag on the executor or something.
	if !cfg.ClusterReset && cfg.ETCDAgent {
		// ETCDAgent is only set to true on servers that are started with --disable-apiserver.
		// In this case, we may be running without an apiserver available in the cluster, and need
		// to wait for one to register and post it's address into APIAddressCh so that we can update
		// the LB proxy with its address.
		if proxy.IsAPIServerLBEnabled() {
			// On RKE2, the agent needs to be started early to run the etcd static pod.
			if err := agent.Agent(ctx, nodeConfig, proxy); err != nil {
				return err
			}
			agentRan = true
		}
		if err := waitForAPIServerAddresses(ctx, nodeConfig, cfg, proxy); err != nil {
			return err
		}
	} else if cfg.ClusterReset && proxy.IsAPIServerLBEnabled() {
		// If we're doing a cluster-reset on RKE2, the kubelet needs to be started early to clean
		// up static pods.
		if err := agent.Agent(ctx, nodeConfig, proxy); err != nil {
			return err
		}
		agentRan = true
	}

	if err := tunnelSetup(ctx, nodeConfig, cfg, proxy); err != nil {
		return err
	}
	if !agentRan {
		return agent.Agent(ctx, nodeConfig, proxy)
	}
	return nil
}

func waitForAPIServerAddresses(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent, proxy proxy.Proxy) error {
	for {
		select {
		case <-time.After(5 * time.Second):
			logrus.Info("Waiting for apiserver addresses")
		case addresses := <-cfg.APIAddressCh:
			for i, a := range addresses {
				host, _, err := net.SplitHostPort(a)
				if err == nil {
					addresses[i] = net.JoinHostPort(host, strconv.Itoa(nodeConfig.ServerHTTPSPort))
					if i == 0 {
						proxy.SetSupervisorDefault(addresses[i])
					}
				}
			}
			proxy.Update(addresses)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// tunnelSetup calls tunnel setup, unless the embedded etc cluster is being reset/restored, in which case
// this is unnecessary as the kubelet is only needed to manage static pods and does not need to establish
// tunneled connections to other cluster members.
func tunnelSetup(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent, proxy proxy.Proxy) error {
	if cfg.ClusterReset {
		return nil
	}
	return tunnel.Setup(ctx, nodeConfig, proxy)
}
