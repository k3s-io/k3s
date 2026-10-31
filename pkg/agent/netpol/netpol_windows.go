package netpol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/hcsshim/hcn"
	agentflannel "github.com/k3s-io/k3s/pkg/agent/flannel"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	networkinginformers "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	windowsFlannelNetwork = "flannel.4096"
	managedACLIDPrefix    = "k3s-netpol-"
	reconcileRetryDelay   = 5 * time.Second
	reconcilePeriod       = 30 * time.Second
)

type endpointACLSetting struct {
	Id              string            `json:"Id,omitempty"`
	Protocols       string            `json:",omitempty"`
	Action          hcn.ActionType    `json:"Action"`
	Direction       hcn.DirectionType `json:"Direction"`
	RemoteAddresses string            `json:",omitempty"`
	LocalPorts      string            `json:",omitempty"`
	RemotePorts     string            `json:",omitempty"`
	RuleType        hcn.RuleType      `json:",omitempty"`
	Priority        uint16            `json:",omitempty"`
}

type windowsController struct {
	podInformer       coreinformers.PodInformer
	namespaceInformer coreinformers.NamespaceInformer
	policyInformer    networkinginformers.NetworkPolicyInformer
	nodeConfig        *daemonconfig.Node
	queue             chan struct{}
}

func Run(ctx context.Context, wg *sync.WaitGroup, nodeConfig *daemonconfig.Node) error {
	if nodeConfig.Flannel.Backend == agentflannel.BackendNone {
		logrus.Warn("Skipping Windows network policy controller: it requires the built-in Flannel win-overlay network")
		return nil
	}
	if nodeConfig.Flannel.Backend != agentflannel.BackendVXLAN {
		return fmt.Errorf("Windows network policy controller only supports the Flannel vxlan backend, got %q", nodeConfig.Flannel.Backend)
	}
	if nodeConfig.Flannel.ConfOverride || nodeConfig.Flannel.CNIConfFile != "" {
		logrus.Warn("Skipping Windows network policy controller: custom Flannel or CNI configuration is not supported")
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

	factory := informers.NewSharedInformerFactory(client, 0)
	controller := &windowsController{
		podInformer:       factory.Core().V1().Pods(),
		namespaceInformer: factory.Core().V1().Namespaces(),
		policyInformer:    factory.Networking().V1().NetworkPolicies(),
		nodeConfig:        nodeConfig,
		queue:             make(chan struct{}, 1),
	}
	for _, informer := range []cache.SharedIndexInformer{
		controller.podInformer.Informer(),
		controller.namespaceInformer.Informer(),
		controller.policyInformer.Informer(),
	} {
		if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(any) { controller.enqueue() },
			UpdateFunc: func(any, any) { controller.enqueue() },
			DeleteFunc: func(any) { controller.enqueue() },
		}); err != nil {
			return fmt.Errorf("registering Windows network policy informer: %w", err)
		}
	}

	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), controller.podInformer.Informer().HasSynced, controller.namespaceInformer.Informer().HasSynced, controller.policyInformer.Informer().HasSynced) {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("waiting for Windows network policy informer caches to sync: %w", err)
		}
		return fmt.Errorf("waiting for Windows network policy informer caches to sync")
	}

	wg.Add(1)
	go controller.run(ctx, wg)
	logrus.Infof("Starting Windows network policy controller for Flannel win-overlay")
	return nil
}

func (c *windowsController) enqueue() {
	select {
	case c.queue <- struct{}{}:
	default:
	}
}

func (c *windowsController) run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(reconcilePeriod)
	defer ticker.Stop()
	c.enqueue()

	var retry <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.enqueue()
		case <-c.queue:
			if err := c.reconcile(); err != nil {
				logrus.WithError(err).Error("Windows network policy reconciliation failed; retrying")
				retry = time.After(reconcileRetryDelay)
			} else {
				retry = nil
			}
		case <-retry:
			retry = nil
			c.enqueue()
		}
	}
}

func (c *windowsController) reconcile() error {
	pods, err := c.podInformer.Lister().List(labels.Everything())
	if err != nil {
		return err
	}
	namespaceList, err := c.namespaceInformer.Lister().List(labels.Everything())
	if err != nil {
		return err
	}
	policies, err := c.policyInformer.Lister().List(labels.Everything())
	if err != nil {
		return err
	}
	namespaces := make(map[string]*corev1.Namespace, len(namespaceList))
	for _, namespace := range namespaceList {
		namespaces[namespace.Name] = namespace
	}

	localPods := make(map[string]*corev1.Pod)
	for _, pod := range pods {
		if pod.Spec.NodeName != c.nodeConfig.AgentConfig.NodeName {
			continue
		}
		for _, address := range podAddresses(pod) {
			if existingPod, ok := localPods[address]; ok && existingPod.UID != pod.UID {
				return fmt.Errorf("local Pods %s/%s and %s/%s share IP address %s",
					existingPod.Namespace, existingPod.Name, pod.Namespace, pod.Name, address)
			}
			localPods[address] = pod
		}
	}

	nodeIPs := make([]string, 0, len(c.nodeConfig.AgentConfig.NodeIPs)+len(c.nodeConfig.AgentConfig.NodeExternalIPs))
	for _, address := range c.nodeConfig.AgentConfig.NodeIPs {
		nodeIPs = append(nodeIPs, address.String())
	}
	for _, address := range c.nodeConfig.AgentConfig.NodeExternalIPs {
		nodeIPs = append(nodeIPs, address.String())
	}

	desiredByUID := make(map[types.UID][]policyRule)
	for _, pod := range localPods {
		if _, exists := desiredByUID[pod.UID]; exists {
			continue
		}
		rules, err := compilePolicyRules(pod, pods, namespaces, nodeIPs, policies)
		if err != nil {
			return err
		}
		desiredByUID[pod.UID] = rules
	}

	network, err := hcn.GetNetworkByName(windowsFlannelNetwork)
	if err != nil {
		return fmt.Errorf("getting HNS network %q: %w", windowsFlannelNetwork, err)
	}
	endpoints, err := hcn.ListEndpointsOfNetwork(network.Id)
	if err != nil {
		return fmt.Errorf("listing HNS endpoints for network %q: %w", windowsFlannelNetwork, err)
	}

	for i := range endpoints {
		endpoint := &endpoints[i]
		var pod *corev1.Pod
		hasIPv4Address := false
		for _, ipConfig := range endpoint.IpConfigurations {
			if ipConfig.IpAddress == "" {
				continue
			}
			address, err := netip.ParseAddr(ipConfig.IpAddress)
			if err != nil {
				if prefix, prefixErr := netip.ParsePrefix(ipConfig.IpAddress); prefixErr == nil {
					address = prefix.Addr()
					err = nil
				}
			}
			if err != nil {
				return fmt.Errorf("parsing IP address %q on HNS endpoint %q: %w", ipConfig.IpAddress, endpoint.Id, err)
			}
			if !address.Is4() {
				continue
			}
			hasIPv4Address = true
			if localPod, ok := localPods[address.String()]; ok {
				pod = localPod
				break
			}
		}
		if !hasIPv4Address {
			continue
		}

		var rules []policyRule
		var uid types.UID
		if pod != nil {
			uid = pod.UID
			rules = desiredByUID[uid]
		}
		desired, err := endpointPolicies(uid, rules)
		if err != nil {
			return fmt.Errorf("building HNS ACLs for endpoint %q: %w", endpoint.Id, err)
		}
		if err := reconcileEndpointPolicies(endpoint, desired); err != nil {
			return fmt.Errorf("reconciling HNS ACLs for endpoint %q: %w", endpoint.Id, err)
		}
	}

	return nil
}

func endpointPolicies(uid types.UID, rules []policyRule) ([]hcn.EndpointPolicy, error) {
	result := make([]hcn.EndpointPolicy, 0, len(rules))
	seen := make(map[string]struct{}, len(rules))
	for _, rule := range rules {
		setting := endpointACLSetting{
			Protocols:       rule.protocol,
			Action:          hcn.ActionType(rule.action),
			Direction:       hcn.DirectionType(rule.direction),
			RemoteAddresses: rule.remoteAddresses,
			RuleType:        hcn.RuleTypeHost,
			Priority:        rule.priority,
		}
		if rule.direction == policyIngress {
			setting.LocalPorts = rule.ports
		} else {
			setting.RemotePorts = rule.ports
		}

		encodedRule, err := json.Marshal(setting)
		if err != nil {
			return nil, fmt.Errorf("marshaling HNS ACL rule: %w", err)
		}
		hash := sha256.Sum256(encodedRule)
		setting.Id = managedACLIDPrefix + string(uid) + "-" + hex.EncodeToString(hash[:8])
		if _, ok := seen[setting.Id]; ok {
			continue
		}
		seen[setting.Id] = struct{}{}

		encodedSetting, err := json.Marshal(setting)
		if err != nil {
			return nil, fmt.Errorf("marshaling HNS ACL setting: %w", err)
		}
		result = append(result, hcn.EndpointPolicy{Type: hcn.ACL, Settings: encodedSetting})
	}
	return result, nil
}

func reconcileEndpointPolicies(endpoint *hcn.HostComputeEndpoint, desired []hcn.EndpointPolicy) error {
	policies, requestType, err := endpointPolicyUpdate(endpoint.Policies, desired)
	if err != nil {
		return err
	}
	if policies == nil {
		return nil
	}
	if err := endpoint.ApplyPolicy(requestType, hcn.PolicyEndpointRequest{Policies: policies}); err != nil {
		return fmt.Errorf("applying HNS endpoint ACL %s: %w", requestType, err)
	}
	return nil
}

func endpointPolicyUpdate(existing, desired []hcn.EndpointPolicy) ([]hcn.EndpointPolicy, hcn.RequestType, error) {
	desiredIDs := make(map[string]struct{}, len(desired))
	for _, policy := range desired {
		id, err := managedACLID(policy)
		if err != nil {
			return nil, "", err
		}
		desiredIDs[id] = struct{}{}
	}

	policies := make([]hcn.EndpointPolicy, 0, len(existing)+len(desired))
	var removed []hcn.EndpointPolicy
	changed := false
	for _, policy := range existing {
		id, err := managedACLID(policy)
		if err != nil {
			return nil, "", err
		}
		if id == "" {
			policies = append(policies, policy)
			continue
		}
		if _, ok := desiredIDs[id]; !ok {
			changed = true
			removed = append(removed, policy)
		}
		delete(desiredIDs, id)
	}

	if !changed && len(desiredIDs) == 0 {
		return nil, "", nil
	}

	// HCN ignores an update without ACLs; remove the existing managed rules instead.
	if len(desired) == 0 {
		return removed, hcn.RequestTypeRemove, nil
	}
	return append(policies, desired...), hcn.RequestTypeUpdate, nil
}

func managedACLID(policy hcn.EndpointPolicy) (string, error) {
	if !strings.EqualFold(string(policy.Type), string(hcn.ACL)) {
		return "", nil
	}
	var setting struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(policy.Settings, &setting); err != nil {
		return "", fmt.Errorf("decoding HNS ACL policy settings: %w", err)
	}
	if strings.HasPrefix(setting.ID, managedACLIDPrefix) {
		return setting.ID, nil
	}
	return "", nil
}
