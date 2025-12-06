package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	systemd "github.com/coreos/go-systemd/v22/daemon"
	"github.com/k3s-io/k3s/pkg/agent/config"
	"github.com/k3s-io/k3s/pkg/agent/containerd"
	"github.com/k3s-io/k3s/pkg/agent/proxy"
	"github.com/k3s-io/k3s/pkg/agent/syssetup"
	"github.com/k3s-io/k3s/pkg/agent/tunnel"
	"github.com/k3s-io/k3s/pkg/certmonitor"
	"github.com/k3s-io/k3s/pkg/cgroups"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	cp "github.com/k3s-io/k3s/pkg/cloudprovider"
	"github.com/k3s-io/k3s/pkg/daemons/agent"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/metrics"
	"github.com/k3s-io/k3s/pkg/nodeconfig"
	"github.com/k3s-io/k3s/pkg/profile"
	"github.com/k3s-io/k3s/pkg/rootless"
	"github.com/k3s-io/k3s/pkg/signals"
	"github.com/k3s-io/k3s/pkg/spegel"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	toolscache "k8s.io/client-go/tools/cache"
	toolswatch "k8s.io/client-go/tools/watch"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/logs"
	app2 "k8s.io/kubernetes/cmd/kube-proxy/app"
	kubeproxyconfig "k8s.io/kubernetes/pkg/proxy/apis/config"
	utilsnet "k8s.io/utils/net"
	utilsptr "k8s.io/utils/ptr"
)

func run(ctx context.Context, cfg cmds.Agent, proxy proxy.Proxy) error {
	nodeConfig, err := config.Get(ctx, cfg, proxy)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to retrieve agent configuration")
	}

	dualCluster, err := utilsnet.IsDualStackCIDRs(nodeConfig.AgentConfig.ClusterCIDRs)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to validate cluster-cidr")
	}
	dualService, err := utilsnet.IsDualStackCIDRs(nodeConfig.AgentConfig.ServiceCIDRs)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to validate service-cidr")
	}
	dualNode, err := utilsnet.IsDualStackIPs(nodeConfig.AgentConfig.NodeIPs)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to validate node-ip")
	}
	serviceIPv4 := utilsnet.IsIPv4CIDR(nodeConfig.AgentConfig.ServiceCIDR)
	clusterIPv4 := utilsnet.IsIPv4CIDR(nodeConfig.AgentConfig.ClusterCIDR)
	nodeIPv4 := utilsnet.IsIPv4String(nodeConfig.AgentConfig.NodeIP)
	serviceIPv6 := utilsnet.IsIPv6CIDR(nodeConfig.AgentConfig.ServiceCIDR)
	clusterIPv6 := utilsnet.IsIPv6CIDR(nodeConfig.AgentConfig.ClusterCIDR)
	nodeIPv6 := utilsnet.IsIPv6String(nodeConfig.AgentConfig.NodeIP)

	// check that cluster-cidr and service-cidr have the same IP versions
	if (serviceIPv6 != clusterIPv6) || (dualCluster != dualService) || (serviceIPv4 != clusterIPv4) {
		return fmt.Errorf("cluster-cidr: %v and service-cidr: %v, must share the same IP version (IPv4, IPv6 or dual-stack)", nodeConfig.AgentConfig.ClusterCIDRs, nodeConfig.AgentConfig.ServiceCIDRs)
	}

	// check that node-ip has the IP versions set in cluster-cidr
	if (clusterIPv6 && !(nodeIPv6 || dualNode)) || (dualCluster && !dualNode) || (clusterIPv4 && !(nodeIPv4 || dualNode)) {
		return fmt.Errorf("cluster-cidr: %v and node-ip: %v, must share the same IP version (IPv4, IPv6 or dual-stack)", nodeConfig.AgentConfig.ClusterCIDRs, nodeConfig.AgentConfig.NodeIPs)
	}

	enableIPv6 := dualCluster || clusterIPv6
	enableIPv4 := dualCluster || clusterIPv4

	// dualStack or IPv6 are not supported on Windows node
	if (goruntime.GOOS == "windows") && enableIPv6 {
		return fmt.Errorf("dual-stack or IPv6 are not supported on Windows node")
	}

	conntrackConfig, err := getConntrackConfig(nodeConfig)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to validate kube-proxy conntrack configuration")
	}
	syssetup.Configure(enableIPv6, conntrackConfig)
	nodeConfig.AgentConfig.EnableIPv4 = enableIPv4
	nodeConfig.AgentConfig.EnableIPv6 = enableIPv6

	if err := executor.Bootstrap(ctx, nodeConfig, cfg); err != nil {
		return err
	}

	if nodeConfig.EmbeddedRegistry {
		if nodeConfig.Docker || nodeConfig.ContainerRuntimeEndpoint != "" {
			return errors.New("embedded registry mirror requires embedded containerd")
		}

		if err := spegel.DefaultRegistry.Start(ctx, nodeConfig, executor.CRIReadyChan()); err != nil {
			return pkgerrors.WithMessage(err, "failed to start embedded registry")
		}
	}

	if nodeConfig.SupervisorMetrics {
		if err := metrics.DefaultMetrics.Start(ctx, nodeConfig); err != nil {
			return pkgerrors.WithMessage(err, "failed to serve metrics")
		}
	}

	if nodeConfig.EnablePProf {
		if err := profile.DefaultProfiler.Start(ctx, nodeConfig); err != nil {
			return pkgerrors.WithMessage(err, "failed to serve pprof")
		}
	}

	if err := setupCriCtlConfig(cfg, nodeConfig); err != nil {
		return err
	}

	// Create a new context to use for agent components that is cancelled on a
	// delay after the signal context. This allows other things (like etcd) to
	// clean up, before agent components exit when their contexts are cancelled.
	ctx = util.DelayCancel(ctx, util.DefaultContextDelay)

	notifySocket := os.Getenv("NOTIFY_SOCKET")
	os.Unsetenv("NOTIFY_SOCKET")

	go func() {
		if err := startCRI(ctx, nodeConfig); err != nil {
			signals.RequestShutdown(pkgerrors.WithMessage(err, "failed to start container runtime"))
		}
	}()

	if err := setupTunnelAndRunAgent(ctx, nodeConfig, cfg, proxy); err != nil {
		return err
	}

	go func() {
		<-executor.APIServerReadyChan()
		if err := startNetwork(ctx, &sync.WaitGroup{}, nodeConfig); err != nil {
			signals.RequestShutdown(pkgerrors.WithMessage(err, "failed to start networking"))
			return
		}

		// By default, the server is responsible for notifying systemd
		// On agent-only nodes, the agent will notify systemd
		if notifySocket != "" {
			logrus.Info(version.Program + " agent is up and running")
			os.Setenv("NOTIFY_SOCKET", notifySocket)
			systemd.SdNotify(true, "READY=1\n")
		}
	}()

	return nil
}

// startCRI starts the configured CRI, or waits for an external CRI to be ready.
func startCRI(ctx context.Context, nodeConfig *daemonconfig.Node) error {
	if nodeConfig.Docker {
		return executor.Docker(ctx, nodeConfig)
	} else if nodeConfig.ContainerRuntimeEndpoint == "" {
		if err := containerd.SetupContainerdConfig(nodeConfig); err != nil {
			return err
		}
		return executor.Containerd(ctx, nodeConfig)
	} else {
		return executor.CRI(ctx, nodeConfig)
	}
}

// startNetwork updates the network annotations on the node and starts the CNI
func startNetwork(ctx context.Context, wg *sync.WaitGroup, nodeConfig *daemonconfig.Node) error {
	// Use the kubelet kubeconfig to update annotations on the local node
	kubeletClient, err := util.GetClientSet(nodeConfig.AgentConfig.KubeConfigKubelet)
	if err != nil {
		return err
	}

	if err := configureNode(ctx, nodeConfig, kubeletClient); err != nil {
		return err
	}

	return executor.CNI(ctx, wg, nodeConfig)
}

// getConntrackConfig uses the kube-proxy code to parse the user-provided kube-proxy-arg values, and
// extract the conntrack settings so that K3s can set them itself. This allows us to soft-fail when
// running K3s in Docker, where kube-proxy is no longer allowed to set conntrack sysctls on newer kernels.
// When running rootless, we do not attempt to set conntrack sysctls - this behavior is copied from kubeadm.
func getConntrackConfig(nodeConfig *daemonconfig.Node) (*kubeproxyconfig.KubeProxyConntrackConfiguration, error) {
	ctConfig := &kubeproxyconfig.KubeProxyConntrackConfiguration{
		MaxPerCore:            utilsptr.To(int32(0)),
		Min:                   utilsptr.To(int32(0)),
		TCPEstablishedTimeout: &metav1.Duration{},
		TCPCloseWaitTimeout:   &metav1.Duration{},
	}

	if nodeConfig.AgentConfig.Rootless {
		return ctConfig, nil
	}

	cmd := app2.NewProxyCommand()
	globalflag.AddGlobalFlags(cmd.Flags(), cmd.Name(), logs.SkipLoggingConfigurationFlags())
	if err := cmd.ParseFlags(util.GetArgs(map[string]string{}, nodeConfig.AgentConfig.ExtraKubeProxyArgs)); err != nil {
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

// RunStandalone bootstraps the executor, but does not run the kubelet or containerd.
// This allows other bits of code that expect the executor to be set up properly to function
// even when the agent is disabled.
func RunStandalone(ctx context.Context, wg *sync.WaitGroup, cfg cmds.Agent) error {
	proxy, err := createProxyAndValidateToken(ctx, &cfg)
	if err != nil {
		return err
	}

	nodeConfig, err := config.Get(ctx, cfg, proxy)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to retrieve agent configuration")
	}

	if err := executor.Bootstrap(ctx, nodeConfig, cfg); err != nil {
		return err
	}

	// this is a no-op just to get the cri ready channel closed
	if err := executor.CRI(ctx, nodeConfig); err != nil {
		return err
	}

	if err := tunnelSetup(ctx, nodeConfig, cfg, proxy); err != nil {
		return err
	}
	if err := certMonitorSetup(ctx, nodeConfig, cfg); err != nil {
		return err
	}

	if nodeConfig.SupervisorMetrics {
		if err := metrics.DefaultMetrics.Start(ctx, nodeConfig); err != nil {
			return pkgerrors.WithMessage(err, "failed to serve metrics")
		}
	}

	if nodeConfig.EnablePProf {
		if err := profile.DefaultProfiler.Start(ctx, nodeConfig); err != nil {
			return pkgerrors.WithMessage(err, "failed to serve pprof")
		}
	}

	return nil
}

// Run sets up cgroups, configures the LB proxy, and triggers startup
// of containerd and kubelet.
func Run(ctx context.Context, wg *sync.WaitGroup, cfg cmds.Agent) error {
	if err := cgroups.Validate(); err != nil {
		return err
	}

	if cfg.Rootless && !cfg.RootlessAlreadyUnshared {
		dualNode, err := utilsnet.IsDualStackIPStrings(cfg.NodeIP.Value())
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
	clientKubeletCert := filepath.Join(agentDir, "client-kubelet.crt")
	clientKubeletKey := filepath.Join(agentDir, "client-kubelet.key")

	if err := os.MkdirAll(agentDir, 0700); err != nil {
		return nil, err
	}

	_, nodeIPs, err := util.GetHostnameAndIPs(cfg.NodeName, cfg.NodeIP.Value())
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "failed to get node name and addresses")
	}

	proxy, err := proxy.NewSupervisorProxy(ctx, !cfg.DisableLoadBalancer, agentDir, cfg.ServerURL, cfg.LBServerPort, utilsnet.IsIPv6(nodeIPs[0]))
	if err != nil {
		return nil, err
	}

	options := []clientaccess.ValidationOption{
		clientaccess.WithUser("node"),
		clientaccess.WithClientCertificate(clientKubeletCert, clientKubeletKey),
	}

	for {
		newToken, err := clientaccess.ParseAndValidateToken(proxy.SupervisorURL(), cfg.Token, options...)
		if err != nil {
			logrus.Errorf("Failed to validate connection to cluster at %s: %v", cfg.ServerURL, err)
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
func configureNode(ctx context.Context, nodeConfig *daemonconfig.Node, coreClient kubernetes.Interface) error {
	patcher := util.NewPatcher[*v1.Node](coreClient.CoreV1().Nodes())
	lw := toolscache.NewListWatchFromClient(coreClient.CoreV1().RESTClient(), "nodes", metav1.NamespaceNone, fields.OneTermEqualSelector(metav1.ObjectNameField, nodeConfig.AgentConfig.NodeName))
	condition := func(ev watch.Event) (bool, error) {
		node, ok := ev.Object.(*v1.Node)
		if !ok {
			return false, errors.New("event object not of type v1.Node")
		}

		patch := util.NewPatchList()
		updateMutableLabels(&nodeConfig.AgentConfig, patch)

		if nodeConfig.AgentConfig.DisableCCM {
			removeAddressAnnotations(patch, node)
			removeLegacyAddressLabels(patch, node)
		} else {
			updateAddressAnnotations(&nodeConfig.AgentConfig, patch, node)
			updateLegacyAddressLabels(&nodeConfig.AgentConfig, patch, node)
		}

		// inject node config
		nodeconfig.SetNodeConfigAnnotations(nodeConfig, patch, node)
		nodeconfig.SetNodeConfigLabels(nodeConfig, patch, node)

		if _, err := patcher.Patch(ctx, patch, nodeConfig.AgentConfig.NodeName); err != nil {
			logrus.Infof("Failed to set annotations and labels on node %s: %v", nodeConfig.AgentConfig.NodeName, err)
			return false, nil
		}
		logrus.Infof("Annotations and labels have been set successfully on node: %s", nodeConfig.AgentConfig.NodeName)
		return true, nil
	}
	if _, err := toolswatch.UntilWithSync(ctx, lw, &v1.Node{}, nil, condition); err != nil {
		return pkgerrors.WithMessage(err, "failed to configure node")
	}
	return nil
}

func updateMutableLabels(agentConfig *daemonconfig.Agent, patch *util.PatchList) {
	for _, m := range agentConfig.NodeLabels {
		var (
			v string
			p = strings.SplitN(m, `=`, 2)
			k = p[0]
		)
		if len(p) > 1 {
			v = p[1]
		}
		patch.Add(v, "metadata", "labels", k)
	}
}

func updateLegacyAddressLabels(agentConfig *daemonconfig.Agent, patch *util.PatchList, node *v1.Node) {
	ls := labels.Set(node.Labels)
	if ls.Has(cp.InternalIPKey) || ls.Has(cp.HostnameKey) {
		patch.Add(agentConfig.NodeIP, "metadata", "labels", cp.InternalIPKey)
		patch.Add(getHostname(agentConfig), "metadata", "labels", cp.HostnameKey)

		if agentConfig.NodeExternalIP != "" {
			patch.Add(agentConfig.NodeExternalIP, "metadata", "labels", cp.ExternalIPKey)
		}
	}
}

func removeLegacyAddressLabels(patch *util.PatchList, node *v1.Node) {
	for _, key := range []string{cp.HostnameKey, cp.InternalIPKey, cp.ExternalIPKey} {
		if _, ok := node.Labels[key]; ok {
			patch.Remove("metadata", "labels", key)
		}
	}
}

// updateAddressAnnotations updates the node annotations with important information about IP addresses of the node
func updateAddressAnnotations(agentConfig *daemonconfig.Agent, patch *util.PatchList, node *v1.Node) {
	patch.Add(util.JoinIPs(agentConfig.NodeIPs), "metadata", "annotations", cp.InternalIPKey)
	patch.Add(getHostname(agentConfig), "metadata", "annotations", cp.HostnameKey)

	if agentConfig.NodeExternalIP != "" {
		patch.Add(util.JoinIPs(agentConfig.NodeExternalIPs), "metadata", "annotations", cp.ExternalIPKey)
	}

	if len(agentConfig.NodeInternalDNSs) > 0 {
		patch.Add(strings.Join(agentConfig.NodeInternalDNSs, ","), "metadata", "annotations", cp.InternalDNSKey)
	} else if _, ok := node.Annotations[cp.InternalDNSKey]; ok {
		patch.Remove("metadata", "annotations", cp.InternalDNSKey)
	}

	if len(agentConfig.NodeExternalDNSs) > 0 {
		patch.Add(strings.Join(agentConfig.NodeExternalDNSs, ","), "metadata", "annotations", cp.ExternalDNSKey)
	} else if _, ok := node.Annotations[cp.ExternalDNSKey]; ok {
		patch.Remove("metadata", "annotations", cp.ExternalDNSKey)
	}
}

func removeAddressAnnotations(patch *util.PatchList, node *v1.Node) {
	for _, key := range []string{cp.HostnameKey, cp.InternalIPKey, cp.ExternalIPKey, cp.InternalDNSKey, cp.ExternalDNSKey} {
		if _, ok := node.Annotations[key]; ok {
			patch.Remove("metadata", "annotations", key)
		}
	}
}

// setupTunnelAndRunAgent starts the agent tunnel, cert expiry monitoring, and
// runs the Agent (cri+kubelet). On etcd-only nodes, an extra goroutine is
// started to retrieve apiserver addresses from the datastore. On other node
// types, this is done later by the tunnel setup, which starts goroutines to
// watch apiserver endpoints.
func setupTunnelAndRunAgent(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent, proxy proxy.Proxy) error {
	// only need to get apiserver addresses from the datastore on an etcd-only node that is not being reset
	if !cfg.ClusterReset && cfg.ETCDAgent {
		go waitForAPIServerAddresses(ctx, nodeConfig, cfg, proxy)
	}

	if err := tunnelSetup(ctx, nodeConfig, cfg, proxy); err != nil {
		return err
	}

	if err := certMonitorSetup(ctx, nodeConfig, cfg); err != nil {
		return err
	}

	return agent.Agent(ctx, nodeConfig, proxy)
}

// waitForAPIServerAddresses syncs apiserver addresses from the datastore. This
// is also handled by the agent tunnel watch, but on etcd-only nodes we need to
// read apiserver addresses from APIAddressCh before the agent has a
// connection to the apiserver. This does not return until addresses or set,
// or the context is cancelled.
func waitForAPIServerAddresses(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent, proxy proxy.Proxy) {
	var localSupervisorDefault bool
	if addresses := proxy.SupervisorAddresses(); len(addresses) > 0 {
		host, _, _ := net.SplitHostPort(addresses[0])
		if host == "127.0.0.1" || host == "::1" {
			localSupervisorDefault = true
		}
	}

	for {
		select {
		case <-time.After(5 * time.Second):
			logrus.Info("Waiting for control-plane node to register apiserver addresses in etcd")
		case addresses := <-cfg.APIAddressCh:
			for i, a := range addresses {
				host, _, err := net.SplitHostPort(a)
				if err == nil {
					addresses[i] = net.JoinHostPort(host, strconv.Itoa(nodeConfig.ServerHTTPSPort))
				}
			}
			// If this is an etcd-only node that started up using its local supervisor,
			// switch to using a control-plane node as the supervisor. Otherwise, leave the
			// configured server address as the default.
			if localSupervisorDefault && len(addresses) > 0 {
				proxy.SetSupervisorDefault(addresses[0])
			}
			proxy.Update(addresses)
			return
		case <-ctx.Done():
			return
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

func certMonitorSetup(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	if cfg.ClusterReset {
		return nil
	}
	return certmonitor.Setup(ctx, nodeConfig, cfg.DataDir)
}

// getHostname returns the actual system hostname.
// If the hostname cannot be determined, or is invalid, the node name is used.
func getHostname(agentConfig *daemonconfig.Agent) string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" || strings.Contains(hostname, "localhost") {
		return agentConfig.NodeName
	}
	return hostname
}
