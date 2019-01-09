package agent

import (
	"context"
	"math/rand"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/util/net"

	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/util/logs"
	app2 "k8s.io/kubernetes/cmd/kube-proxy/app"
	"k8s.io/kubernetes/cmd/kubelet/app"
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus" // for client metric registration
	_ "k8s.io/kubernetes/pkg/version/prometheus"        // for version metric registration
)

func Agent(config *config.Agent) error {
	rand.Seed(time.Now().UTC().UnixNano())

	kubelet(config)
	kubeProxy(config)

	return nil
}

func kubeProxy(config *config.Agent) {
	args := []string{
		"--proxy-mode", "iptables",
		"--healthz-bind-address", "127.0.0.1",
		"--kubeconfig", config.KubeConfig,
		"--cluster-cidr", config.ClusterCIDR.String(),
	}
	args = append(args, config.ExtraKubeletArgs...)

	command := app2.NewProxyCommand()
	command.SetArgs(args)
	go func() {
		err := command.Execute()
		logrus.Fatalf("kube-proxy exited: %v", err)
	}()
}

func kubelet(cfg *config.Agent) {
	command := app.NewKubeletCommand(context.Background().Done())
	logs.InitLogs()
	defer logs.FlushLogs()

	args := []string{
		"--healthz-bind-address", "127.0.0.1",
		"--read-only-port", "0",
		"--allow-privileged=true",
		"--cluster-domain", "cluster.local",
		"--kubeconfig", cfg.KubeConfig,
		"--eviction-hard", "imagefs.available<5%,nodefs.available<5%",
		"--eviction-minimum-reclaim", "imagefs.available=10%,nodefs.available=10%",
		"--node-ip", cfg.NodeIP,
		"--fail-swap-on=false",
		//"--cgroup-root", "/k3s",
		"--cgroup-driver", "cgroupfs",
	}
	if cfg.RootDir != "" {
		args = append(args, "--root-dir", cfg.RootDir)
		args = append(args, "--cert-dir", filepath.Join(cfg.RootDir, "pki"))
		args = append(args, "--seccomp-profile-root", filepath.Join(cfg.RootDir, "seccomp"))
	}
	if cfg.CNIConfDir != "" {
		args = append(args, "--cni-conf-dir", cfg.CNIConfDir)
	}
	if cfg.CNIBinDir != "" {
		args = append(args, "--cni-bin-dir", cfg.CNIBinDir)
	}
	if len(cfg.ClusterDNS) > 0 {
		args = append(args, "--cluster-dns", cfg.ClusterDNS.String())
	}
	if cfg.RuntimeSocket != "" {
		args = append(args, "--container-runtime", "remote")
		args = append(args, "--container-runtime-endpoint", cfg.RuntimeSocket)
	}
	if cfg.ListenAddress != "" {
		args = append(args, "--address", cfg.ListenAddress)
	}
	if cfg.CACertPath != "" {
		args = append(args, "--anonymous-auth=false", "--client-ca-file", cfg.CACertPath)
	}
	if cfg.NodeName != "" {
		args = append(args, "--hostname-override", cfg.NodeName)
	}
	defaultIP, err := net.ChooseHostInterface()
	if err != nil || defaultIP.String() != cfg.NodeIP {
		args = append(args, "--node-ip", cfg.NodeIP)
	}
	args = append(args, cfg.ExtraKubeletArgs...)

	command.SetArgs(args)

	go func() {
		logrus.Infof("Running kubelet %s", config.ArgString(args))
		logrus.Fatalf("kubelet exited: %v", command.Execute())
	}()
}
