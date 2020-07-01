// +build !windows

package netpol

import (
	"context"
	"time"

	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
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

	// retry backoff to wait for the clusterrolebinding of user "system:k3s-controller"
	retryBackoff := wait.Backoff{
		Steps:    6,
		Duration: 100 * time.Millisecond,
		Factor:   3.0,
		Cap:      30 * time.Second,
	}
	retryErr := retry.OnError(retryBackoff, errors.IsForbidden, func() error {
		_, err := client.NetworkingV1().NetworkPolicies("").List(ctx, metav1.ListOptions{})
		return err
	})
	if retryErr != nil {
		return retryErr
	}

	npc, err := NewNetworkPolicyController(ctx.Done(), client, time.Minute, nodeConfig.AgentConfig.NodeName)
	if err != nil {
		return err
	}

	go npc.Run(ctx.Done())

	return nil
}
