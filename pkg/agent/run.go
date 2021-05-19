package agent

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	systemd "github.com/coreos/go-systemd/daemon"
	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/agent/config"
	"github.com/rancher/k3s/pkg/agent/containerd"
	"github.com/rancher/k3s/pkg/agent/flannel"
	"github.com/rancher/k3s/pkg/agent/loadbalancer"
	"github.com/rancher/k3s/pkg/agent/netpol"
	"github.com/rancher/k3s/pkg/agent/syssetup"
	"github.com/rancher/k3s/pkg/agent/tunnel"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/daemons/agent"
	daemonconfig "github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/nodeconfig"
	"github.com/rancher/k3s/pkg/rootless"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	app2 "k8s.io/kubernetes/cmd/kube-proxy/app"
	kubeproxyconfig "k8s.io/kubernetes/pkg/proxy/apis/config"
	utilpointer "k8s.io/utils/pointer"
)

const (
	InternalIPLabel = "k3s.io/internal-ip"
	ExternalIPLabel = "k3s.io/external-ip"
	HostnameLabel   = "k3s.io/hostname"
)

func run(ctx context.Context, cfg cmds.Agent, lb *loadbalancer.LoadBalancer) error {
	nodeConfig := config.Get(ctx, cfg)

	conntrackConfig, err := getConntrackConfig(nodeConfig)
	if err != nil {
		return errors.Wrap(err, "failed to validate kube-proxy conntrack configuration")
	}
	syssetup.Configure(conntrackConfig)

	if !nodeConfig.NoFlannel {
		if err := flannel.Prepare(ctx, nodeConfig); err != nil {
			return err
		}
	}

	if !nodeConfig.Docker && nodeConfig.ContainerRuntimeEndpoint == "" {
		if err := containerd.Run(ctx, nodeConfig); err != nil {
			return err
		}
	}

	if err := tunnel.Setup(ctx, nodeConfig, lb.Update); err != nil {
		return err
	}

	if err := agent.Agent(&nodeConfig.AgentConfig); err != nil {
		return err
	}

	coreClient, err := coreClient(nodeConfig.AgentConfig.KubeConfigKubelet)
	if err != nil {
		return err
	}
	if !nodeConfig.NoFlannel {
		if err := flannel.Run(ctx, nodeConfig, coreClient.CoreV1().Nodes()); err != nil {
			return err
		}
	}

	if err := configureNode(ctx, &nodeConfig.AgentConfig, coreClient.CoreV1().Nodes()); err != nil {
		return err
	}

	if !nodeConfig.AgentConfig.DisableNPC {
		if err := netpol.Run(ctx, nodeConfig); err != nil {
			return err
		}
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
	if err := cmd.ParseFlags(daemonconfig.GetArgsList(map[string]string{}, nodeConfig.AgentConfig.ExtraKubeProxyArgs)); err != nil {
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

func Run(ctx context.Context, cfg cmds.Agent) error {
	if err := validate(); err != nil {
		return err
	}

	if cfg.Rootless && !cfg.RootlessAlreadyUnshared {
		if err := rootless.Rootless(cfg.DataDir); err != nil {
			return err
		}
	}

	cfg.DataDir = filepath.Join(cfg.DataDir, "agent")
	os.MkdirAll(cfg.DataDir, 0700)

	lb, err := loadbalancer.Setup(ctx, cfg)
	if err != nil {
		return err
	}
	if lb != nil {
		cfg.ServerURL = lb.LoadBalancerServerURL()
	}

	for {
		newToken, err := clientaccess.NormalizeAndValidateTokenForUser(cfg.ServerURL, cfg.Token, "node")
		if err != nil {
			logrus.Error(err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}
		cfg.Token = newToken
		break
	}

	systemd.SdNotify(true, "READY=1\n")
	return run(ctx, cfg, lb)
}

func validate() error {
	cgroups, err := ioutil.ReadFile("/proc/self/cgroup")
	if err != nil {
		return err
	}

	if !strings.Contains(string(cgroups), "cpuset") {
		logrus.Warn("Failed to find cpuset cgroup, you may need to add \"cgroup_enable=cpuset\" to your linux cmdline (/boot/cmdline.txt on a Raspberry Pi)")
	}

	if !strings.Contains(string(cgroups), "memory") {
		msg := "ailed to find memory cgroup, you may need to add \"cgroup_memory=1 cgroup_enable=memory\" to your linux cmdline (/boot/cmdline.txt on a Raspberry Pi)"
		logrus.Error("F" + msg)
		return errors.New("f" + msg)
	}

	return nil
}

func configureNode(ctx context.Context, agentConfig *daemonconfig.Agent, nodes v1.NodeInterface) error {
	for {
		node, err := nodes.Get(ctx, agentConfig.NodeName, metav1.GetOptions{})
		if err != nil {
			logrus.Infof("Waiting for kubelet to be ready on node %s: %v", agentConfig.NodeName, err)
			time.Sleep(1 * time.Second)
			continue
		}

		newLabels, updateMutables := updateMutableLabels(agentConfig, node.Labels)

		updateAddresses := !agentConfig.DisableCCM
		if updateAddresses {
			newLabels, updateAddresses = updateAddressLabels(agentConfig, newLabels)
		}

		// inject node config
		updateNode, err := nodeconfig.SetNodeConfigAnnotations(node)
		if err != nil {
			return err
		}
		if updateAddresses || updateMutables {
			node.Labels = newLabels
			updateNode = true
		}
		if updateNode {
			if _, err := nodes.Update(ctx, node, metav1.UpdateOptions{}); err != nil {
				logrus.Infof("Failed to update node %s: %v", agentConfig.NodeName, err)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second):
					continue
				}
			}
			logrus.Infof("labels have been set successfully on node: %s", agentConfig.NodeName)
		} else {
			logrus.Infof("labels have already set on node: %s", agentConfig.NodeName)
		}

		break
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

func updateAddressLabels(agentConfig *daemonconfig.Agent, nodeLabels map[string]string) (map[string]string, bool) {
	result := map[string]string{
		InternalIPLabel: agentConfig.NodeIP,
		HostnameLabel:   agentConfig.NodeName,
	}

	if agentConfig.NodeExternalIP != "" {
		result[ExternalIPLabel] = agentConfig.NodeExternalIP
	}

	result = labels.Merge(nodeLabels, result)
	return result, !equality.Semantic.DeepEqual(nodeLabels, result)
}
