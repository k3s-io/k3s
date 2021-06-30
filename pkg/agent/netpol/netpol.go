// Apache License v2.0 (copyright Cloud Native Labs & Rancher Labs)
// - modified from https://github.com/cloudnativelabs/kube-router/blob/73b1b03b32c5755b240f6c077bb097abe3888314/pkg/controllers/netpol.go

// +build !windows

package netpol

import (
	"context"
	"sync"

	"github.com/rancher/k3s/pkg/agent/netpol/utils"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Run creates and starts a new instance of the kube-router network policy controller
// The code in this function is cribbed from the upstream controller at:
// https://github.com/cloudnativelabs/kube-router/blob/ee9f6d890d10609284098229fa1e283ab5d83b93/pkg/cmd/kube-router.go#L78
// The NewNetworkPolicyController function has also been modified to use the k3s config.Node struct instead of KubeRouter's
// CLI configuration, eliminate use of a WaitGroup for shutdown sequencing, and drop Prometheus metrics support.
func Run(ctx context.Context, nodeConfig *config.Node) error {
	set, err := utils.NewIPSet(false)
	if err != nil {
		logrus.Warnf("Skipping network policy controller start, ipset unavailable: %v", err)
		return nil
	}

	if err := set.Save(); err != nil {
		logrus.Warnf("Skipping network policy controller start, ipset save failed: %v", err)
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

	stopCh := ctx.Done()
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	podInformer := informerFactory.Core().V1().Pods().Informer()
	nsInformer := informerFactory.Core().V1().Namespaces().Informer()
	npInformer := informerFactory.Networking().V1().NetworkPolicies().Informer()
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	npc, err := NewNetworkPolicyController(client, nodeConfig, podInformer, npInformer, nsInformer, &sync.Mutex{})
	if err != nil {
		return err
	}

	podInformer.AddEventHandler(npc.PodEventHandler)
	nsInformer.AddEventHandler(npc.NamespaceEventHandler)
	npInformer.AddEventHandler(npc.NetworkPolicyEventHandler)

	go npc.Run(stopCh)

	return nil
}
