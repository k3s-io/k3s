package main

import (
	"context"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/cmd/agent"
	"k8s.io/kubernetes/cmd/server"
)

func runAgent() {
	_, ipNet, err := net.ParseCIDR("10.43.0.0/16")
	if err != nil {
		panic(err)
	}

	err = agent.Agent(&agent.AgentConfig{
		KubeConfig:    "./data/cred/kubeconfig-node.yaml",
		RuntimeSocket: "unix:///run/containerd/containerd.sock",
		ClusterCIDR:   *ipNet,
	})
	logrus.Fatal(err)
}

func runServer() {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	_, cidr, err := net.ParseCIDR("10.43.0.0/16")
	if err != nil {
		panic(err)
	}

	_, clusterCidr, err := net.ParseCIDR("10.42.0.0/16")
	if err != nil {
		panic(err)
	}
	cfg := server.ServerConfig{
		DataDir:        "./data",
		ListenAddr:     net.ParseIP("0.0.0.0"),
		ListenPort:     6443,
		ClusterIPRange: *clusterCidr,
		ServiceIPRange: *cidr,
		PublicHostname: hostname,
	}

	ctx := context.Background()
	err = server.Server(ctx, &cfg)
	if err != nil {
		panic(err)
	}

	<-ctx.Done()

}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "agent" {
		runAgent()
	}
	runServer()
}
