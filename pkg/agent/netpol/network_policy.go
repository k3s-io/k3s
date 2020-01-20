package netpol

import (
	"context"
	"time"

	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func Run(ctx context.Context, nodeConfig *config.Node) error {
	if _, err := NewSavedIPSet(false); err != nil {
		logrus.Warnf("Skipping network policy controller start, ipset unavailable: %v", err)
		return nil
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", nodeConfig.AgentConfig.KubeConfigK3sController)
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
