package agent

import (
	"math/rand"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/util/logs"
	app2 "k8s.io/kubernetes/cmd/kube-proxy/app"
	"k8s.io/kubernetes/cmd/kubelet/app"
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus" // for client metric registration
	_ "k8s.io/kubernetes/pkg/version/prometheus"        // for version metric registration
)

type AgentConfig struct {
	NodeName      string
	ClusterCIDR   net.IPNet
	KubeConfig    string
	RuntimeSocket string
	ListenAddress string
	CACertPath    string
	CNIBinDir     string
	CNIConfDir    string
}

func Agent(config *AgentConfig) error {
	rand.Seed(time.Now().UTC().UnixNano())

	prepare(config)

	kubelet(config)
	kubeProxy(config)

	return nil
}

func prepare(config *AgentConfig) {
	if config.CNIBinDir == "" {
		config.CNIBinDir = "/opt/cni/bin"
	}
	if config.CNIConfDir == "" {
		config.CNIConfDir = "/etc/cni/net.d"
	}
}

func kubeProxy(config *AgentConfig) {
	command := app2.NewProxyCommand()
	command.SetArgs([]string{
		"--proxy-mode", "iptables",
		"--healthz-bind-address", "127.0.0.1",
		"--kubeconfig", config.KubeConfig,
		"--cluster-cidr", config.ClusterCIDR.String(),
	})

	go func() {
		err := command.Execute()
		logrus.Fatalf("kube-proxy exited: %v", err)
	}()
}

func kubelet(config *AgentConfig) {
	command := app.NewKubeletCommand()
	logs.InitLogs()
	defer logs.FlushLogs()

	args := []string{
		"--healthz-bind-address", "127.0.0.1",
		"--read-only-port", "0",
		"--allow-privileged=true",
		"--cluster-domain", "cluster.local",
		"--cluster-dns", "10.43.0.10",
		"--kubeconfig", config.KubeConfig,
		"--eviction-hard", "imagefs.available<5%,nodefs.available<5%",
		"--eviction-minimum-reclaim", "imagefs.available=10%,nodefs.available=10%",
		"--fail-swap-on=false",
		"--cgroup-root", "/k3s",
		"--cgroup-driver", "cgroupfs",
		"--container-runtime-endpoint", config.RuntimeSocket,
		"--cni-conf-dir", config.CNIConfDir,
		"--cni-bin-dir", config.CNIBinDir,
	}
	if config.ListenAddress != "" {
		args = append(args, "--address", config.ListenAddress)
	}
	if config.CACertPath != "" {
		args = append(args, "--anonymous-auth=false", "--client-ca-file", config.CACertPath)
	}
	if config.NodeName != "" {
		args = append(args, "--hostname-override", config.NodeName)
	}

	command.SetArgs(args)

	go func() {
		logrus.Fatalf("kubelet exited: %v", command.Execute())
	}()
}
