// Apache License v2.0 (copyright Cloud Native Labs & Rancher Labs)
// - modified from https://github.com/cloudnativelabs/kube-router/blob/73b1b03b32c5755b240f6c077bb097abe3888314/pkg/controllers/netpol.go

//go:build !windows
// +build !windows

package netpol

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	cloudproviderapi "k8s.io/cloud-provider/api"

	"github.com/cloudnativelabs/kube-router/v2/pkg/controllers/netpol"
	"github.com/cloudnativelabs/kube-router/v2/pkg/healthcheck"
	krmetrics "github.com/cloudnativelabs/kube-router/v2/pkg/metrics"
	"github.com/cloudnativelabs/kube-router/v2/pkg/options"
	"github.com/cloudnativelabs/kube-router/v2/pkg/utils"
	"github.com/cloudnativelabs/kube-router/v2/pkg/version"
	"github.com/coreos/go-iptables/iptables"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/metrics"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1core "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	// ensure that kube-router exposes metrics through the same registry used by Kubernetes components
	krmetrics.DefaultRegisterer = metrics.DefaultRegisterer
	krmetrics.DefaultGatherer = metrics.DefaultGatherer
}

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

	// kube-router netpol requires addresses to be available in the node object.
	// Wait until the uninitialized taint has been removed, at which point the addresses should be set.
	// TODO: Replace with non-deprecated PollUntilContextTimeout when our and Kubernetes code migrate to it
	if err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		// Get the node object
		node, err := client.CoreV1().Nodes().Get(ctx, nodeConfig.AgentConfig.NodeName, metav1.GetOptions{})
		if err != nil {
			logrus.Infof("Network policy controller waiting to get Node %s: %v", nodeConfig.AgentConfig.NodeName, err)
			return false, nil
		}
		// Check for the taint that should be removed by cloud-provider when the node has been initialized.
		for _, taint := range node.Spec.Taints {
			if taint.Key == cloudproviderapi.TaintExternalCloudProvider {
				logrus.Infof("Network policy controller waiting for removal of %s taint", cloudproviderapi.TaintExternalCloudProvider)
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return errors.Wrapf(err, "network policy controller failed to wait for %s taint to be removed from Node %s", cloudproviderapi.TaintExternalCloudProvider, nodeConfig.AgentConfig.NodeName)
	}

	krConfig := options.NewKubeRouterConfig()
	var serviceIPs []string
	for _, elem := range nodeConfig.AgentConfig.ServiceCIDRs {
		serviceIPs = append(serviceIPs, elem.String())
	}
	krConfig.ClusterIPCIDRs = serviceIPs
	krConfig.EnableIPv4 = nodeConfig.AgentConfig.EnableIPv4
	krConfig.EnableIPv6 = nodeConfig.AgentConfig.EnableIPv6
	krConfig.NodePortRange = strings.ReplaceAll(nodeConfig.AgentConfig.ServiceNodePortRange.String(), "-", ":")
	krConfig.HostnameOverride = nodeConfig.AgentConfig.NodeName
	krConfig.MetricsEnabled = true
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

	// Start kube-router healthcheck controller; netpol requires it
	hc, err := healthcheck.NewHealthController(krConfig)
	if err != nil {
		return err
	}

	// Start kube-router metrics controller to avoid complaints about metrics heartbeat missing
	mc, err := krmetrics.NewMetricsController(krConfig)
	if err != nil {
		return nil
	}

	// Initialize all healthcheck timers. Otherwise, the system reports heartbeat missing messages
	hc.SetAlive()

	wg.Add(1)
	go hc.RunCheck(healthCh, stopCh, &wg)

	wg.Add(1)
	go metricsRunCheck(mc, healthCh, stopCh, &wg)

	npc, err := netpol.NewNetworkPolicyController(client, krConfig, podInformer, npInformer, nsInformer, &sync.Mutex{},
		iptablesCmdHandlers, ipSetHandlers)
	if err != nil {
		return errors.Wrap(err, "unable to initialize network policy controller")
	}

	podInformer.AddEventHandler(npc.PodEventHandler)
	nsInformer.AddEventHandler(npc.NamespaceEventHandler)
	npInformer.AddEventHandler(npc.NetworkPolicyEventHandler)

	wg.Add(1)
	logrus.Infof("Starting network policy controller version %s, built on %s, %s", version.Version, version.BuildDate, runtime.Version())
	go npc.Run(healthCh, stopCh, &wg)

	return nil
}

// metricsRunCheck is a stub version of mc.Run() that doesn't start up a dedicated http server.
func metricsRunCheck(mc *krmetrics.Controller, healthChan chan<- *healthcheck.ControllerHeartbeat, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	t := time.NewTicker(3 * time.Second)
	defer wg.Done()

	// register metrics for this controller
	krmetrics.BuildInfo.WithLabelValues(runtime.Version(), version.Version).Set(1)
	krmetrics.DefaultRegisterer.MustRegister(krmetrics.BuildInfo)

	for {
		healthcheck.SendHeartBeat(healthChan, "MC")
		select {
		case <-stopCh:
			t.Stop()
			return
		case <-t.C:
			logrus.Debugf("Kube-router network policy controller metrics tick")
		}
	}
}
