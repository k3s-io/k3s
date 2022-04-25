// Apache License v2.0 (copyright Cloud Native Labs & Rancher Labs)
// - modified from https://github.com/cloudnativelabs/kube-router/blob/73b1b03b32c5755b240f6c077bb097abe3888314/pkg/controllers/netpol.go

//go:build !windows
// +build !windows

package netpol

import (
	"context"
	"strings"
	"sync"

	"github.com/cloudnativelabs/kube-router/pkg/controllers/netpol"
	"github.com/cloudnativelabs/kube-router/pkg/healthcheck"
	"github.com/cloudnativelabs/kube-router/pkg/options"
	"github.com/cloudnativelabs/kube-router/pkg/utils"
	"github.com/coreos/go-iptables/iptables"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1core "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Run creates and starts a new instance of the kube-router network policy controller
// The code in this function is cribbed from the upstream controller at:
// https://github.com/cloudnativelabs/kube-router/blob/ee9f6d890d10609284098229fa1e283ab5d83b93/pkg/cmd/kube-router.go#L78
// It converts the k3s config.Node into kube-router configuration (only the
// subset of options needed for netpol controller).
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

	krConfig := options.NewKubeRouterConfig()
	krConfig.ClusterIPCIDR = util.JoinIPNets(nodeConfig.AgentConfig.ServiceCIDRs)
	krConfig.EnableIPv4 = nodeConfig.AgentConfig.EnableIPv4
	krConfig.EnableIPv6 = nodeConfig.AgentConfig.EnableIPv6
	krConfig.NodePortRange = strings.ReplaceAll(nodeConfig.AgentConfig.ServiceNodePortRange.String(), "-", ":")
	krConfig.HostnameOverride = nodeConfig.AgentConfig.NodeName
	krConfig.MetricsEnabled = false
	krConfig.RunFirewall = true
	krConfig.RunRouter = false
	krConfig.RunServiceProxy = false

	stopCh := ctx.Done()
	healthCh := make(chan *healthcheck.ControllerHeartbeat)

	// We don't use this WaitGroup, but kube-router components require it.
	var wg sync.WaitGroup

	informerFactory := informers.NewSharedInformerFactory(client, 0)
	podInformer := informerFactory.Core().V1().Pods().Informer()
	nsInformer := informerFactory.Core().V1().Namespaces().Informer()
	npInformer := informerFactory.Networking().V1().NetworkPolicies().Informer()
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	iptablesCmdHandlers := make(map[v1core.IPFamily]utils.IPTablesHandler, 2)
	ipSetHandlers := make(map[v1core.IPFamily]utils.IPSetHandler, 2)

	if nodeConfig.AgentConfig.EnableIPv4 {
		iptHandler, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
		if err != nil {
			return errors.Wrap(err, "failed to create iptables handler")
		}
		iptablesCmdHandlers[v1core.IPv4Protocol] = iptHandler

		ipset, err := utils.NewIPSet(false)
		if err != nil {
			return errors.Wrap(err, "failed to create ipset handler")
		}
		ipSetHandlers[v1core.IPv4Protocol] = ipset
	}

	if nodeConfig.AgentConfig.EnableIPv6 {
		ipt6Handler, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
		if err != nil {
			return errors.Wrap(err, "failed to create iptables handler")
		}
		iptablesCmdHandlers[v1core.IPv6Protocol] = ipt6Handler

		ipset, err := utils.NewIPSet(true)
		if err != nil {
			return errors.Wrap(err, "failed to create ipset handler")
		}
		ipSetHandlers[v1core.IPv6Protocol] = ipset
	}

	// Start kube-router healthcheck server. Netpol requires it
	hc, err := healthcheck.NewHealthController(krConfig)
	if err != nil {
		return err
	}

	// Initialize all healthcheck timers. Otherwise, the system reports incorrect heartbeat missing messages
	hc.SetAlive()

	wg.Add(1)
	go hc.RunCheck(healthCh, stopCh, &wg)

	npc, err := netpol.NewNetworkPolicyController(client, krConfig, podInformer, npInformer, nsInformer, &sync.Mutex{},
		iptablesCmdHandlers, ipSetHandlers)
	if err != nil {
		return err
	}

	podInformer.AddEventHandler(npc.PodEventHandler)
	nsInformer.AddEventHandler(npc.NamespaceEventHandler)
	npInformer.AddEventHandler(npc.NetworkPolicyEventHandler)

	wg.Add(1)
	logrus.Info("Starting the netpol controller")
	go npc.Run(healthCh, stopCh, &wg)

	return nil
}
