// Apache License v2.0 (copyright Cloud Native Labs & Rancher Labs)
// - modified from https://github.com/cloudnativelabs/kube-router/blob/73b1b03b32c5755b240f6c077bb097abe3888314/pkg/controllers/netpol.go

//go:build !windows

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
	"github.com/cloudnativelabs/kube-router/v2/pkg/svcip"
	"github.com/cloudnativelabs/kube-router/v2/pkg/utils"
	"github.com/cloudnativelabs/kube-router/v2/pkg/version"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/metrics"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/util/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type sharedInformers struct {
	pods            cache.SharedIndexInformer
	namespaces      cache.SharedIndexInformer
	networkPolicies cache.SharedIndexInformer
}

func (s *sharedInformers) Pods() cache.SharedIndexInformer            { return s.pods }
func (s *sharedInformers) Namespaces() cache.SharedIndexInformer      { return s.namespaces }
func (s *sharedInformers) NetworkPolicies() cache.SharedIndexInformer { return s.networkPolicies }

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
func Run(ctx context.Context, wg *sync.WaitGroup, nodeConfig *config.Node) error {
	set, err := utils.NewIPSet(false)
	if err != nil {
		logrus.Warnf("Skipping network policy controller start, ipset unavailable: %v", err)
		return nil
	}

	if err := set.Save(); err != nil {
		logrus.Warnf("Skipping network policy controller start, ipset save failed: %v", err)
		return nil
	}

	restConfig, err := util.GetRESTConfig(nodeConfig.AgentConfig.KubeConfigK3sController)
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	// kube-router netpol requires addresses to be available in the node object.
	// Wait until the ready condition is updated and the uninitialized taint has
	// been removed, at which point the addresses should be synced.
	startTime := time.Now().Truncate(time.Second)
	if err := wait.PollUntilContextTimeout(ctx, 2*time.Second, util.DefaultAPIServerReadyTimeout, true, func(ctx context.Context) (bool, error) {
		var readyTime metav1.Time
		// Get the node object
		node, err := client.CoreV1().Nodes().Get(ctx, nodeConfig.AgentConfig.NodeName, metav1.GetOptions{})
		if err != nil {
			logrus.Infof("Network policy controller waiting to get Node %s: %v", nodeConfig.AgentConfig.NodeName, err)
			return false, nil
		}
		for _, cond := range node.Status.Conditions {
			if cond.Type == v1.NodeReady && cond.Status == v1.ConditionTrue {
				readyTime = cond.LastHeartbeatTime
			}
		}
		if readyTime.Time.Before(startTime) {
			logrus.Debugf("Waiting for Ready condition to be updated for network policy controller")
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
		return errors.WithMessagef(err, "network policy controller failed to wait for %s taint to be removed from Node %s", cloudproviderapi.TaintExternalCloudProvider, nodeConfig.AgentConfig.NodeName)
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
	krConfig.NetPolDefaultDeny = true
	krConfig.MetricsEnabled = true
	krConfig.RunFirewall = true
	krConfig.RunRouter = false
	krConfig.RunServiceProxy = false
	krConfig.StrictExternalIPValidation = false
	krConfig.UseNftablesForNetpol = false

	stopCh := ctx.Done()
	healthCh := make(chan *healthcheck.ControllerHeartbeat)

	informerFactory := informers.NewSharedInformerFactory(client, 0)
	krInformers := &sharedInformers{
		pods:            informerFactory.Core().V1().Pods().Informer(),
		namespaces:      informerFactory.Core().V1().Namespaces().Informer(),
		networkPolicies: informerFactory.Networking().V1().NetworkPolicies().Informer(),
	}
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	iptablesCmdHandlers, ipSetHandlers, err := netpol.NewIPTablesHandlers(krConfig)
	if err != nil {
		return errors.WithMessage(err, "failed to create iptables handler")
	}

	ipValidator, err := svcip.NewValidator(svcip.Config{
		ExternalIPCIDRs:   krConfig.ExternalIPCIDRs,
		LoadBalancerCIDRs: krConfig.LoadBalancerCIDRs,
		ClusterIPCIDRs:    krConfig.ClusterIPCIDRs,
		StrictValidation:  krConfig.StrictExternalIPValidation,
		EnableIPv4:        krConfig.EnableIPv4,
		EnableIPv6:        krConfig.EnableIPv6,
	})

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
	go hc.RunCheck(healthCh, stopCh, wg)

	wg.Add(1)
	go metricsRunCheck(mc, healthCh, stopCh, wg)

	npc, err := netpol.NewNetworkPolicyController(client, krConfig, krInformers, &sync.Mutex{}, nil,
		iptablesCmdHandlers, ipSetHandlers, ipValidator, nil)
	if err != nil {
		return errors.WithMessage(err, "unable to initialize network policy controller")
	}

	wg.Add(1)
	logrus.Infof("Starting network policy controller version %s, built on %s, %s", version.Version, version.BuildDate, runtime.Version())
	go npc.Run(healthCh, stopCh, wg)

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
		healthcheck.SendHeartBeat(healthChan, healthcheck.MetricsController)
		select {
		case <-stopCh:
			t.Stop()
			return
		case <-t.C:
			logrus.Debugf("Kube-router network policy controller metrics tick")
		}
	}
}
