package netpol

import (
	"context"
	"time"

	"github.com/rancher/k3s/pkg/daemons/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func Run(ctx context.Context, nodeConfig *config.Node) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", nodeConfig.AgentConfig.KubeConfigNode)
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	npc, err := NewNetworkPolicyController(ctx.Done(), client, time.Minute, nodeConfig.AgentConfig.NodeName)
	if err != nil {
		return err
	}

	go npc.Run(ctx.Done())

	return nil
}
