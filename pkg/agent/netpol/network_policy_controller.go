// Apache License v2.0 (copyright Cloud Native Labs & Rancher Labs)
// - modified from https://github.com/cloudnativelabs/kube-router/blob/ee9f6d890d10609284098229fa1e283ab5d83b93/pkg/controllers/netpol/network_policy_controller.go

// +build !windows

package netpol

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/rancher/k3s/pkg/agent/netpol/utils"
	"github.com/rancher/k3s/pkg/daemons/config"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	glog "k8s.io/klog"
)

const (
	kubePodFirewallChainPrefix   = "KUBE-POD-FW-"
	kubeNetworkPolicyChainPrefix = "KUBE-NWPLCY-"
	kubeSourceIPSetPrefix        = "KUBE-SRC-"
	kubeDestinationIPSetPrefix   = "KUBE-DST-"
	kubeInputChainName           = "KUBE-ROUTER-INPUT"
	kubeForwardChainName         = "KUBE-ROUTER-FORWARD"
	kubeOutputChainName          = "KUBE-ROUTER-OUTPUT"
	defaultSyncPeriod            = 5 * time.Minute
)

// Network policy controller provides both ingress and egress filtering for the pods as per the defined network
// policies. Two different types of iptables chains are used. Each pod running on the node which either
// requires ingress or egress filtering gets a pod specific chains. Each network policy has a iptables chain, which
// has rules expressed through ipsets matching source and destination pod ip's. In the FORWARD chain of the
// filter table a rule is added to jump the traffic originating (in case of egress network policy) from the pod
// or destined (in case of ingress network policy) to the pod specific iptables chain. Each
// pod specific iptables chain has rules to jump to the network polices chains, that pod matches. So packet
// originating/destined from/to pod goes through filter table's, FORWARD chain, followed by pod specific chain,
// followed by one or more network policy chains, till there is a match which will accept the packet, or gets
// dropped by the rule in the pod chain, if there is no match.

// NetworkPolicyController strcut to hold information required by NetworkPolicyController
type NetworkPolicyController struct {
	nodeIP                  net.IP
	nodeHostName            string
	serviceClusterIPRange   net.IPNet
	serviceExternalIPRanges []net.IPNet
	serviceNodePortRange    string
	mu                      sync.Mutex
	syncPeriod              time.Duration
	fullSyncRequestChan     chan struct{}

	ipSetHandler *utils.IPSet

	podLister cache.Indexer
	npLister  cache.Indexer
	nsLister  cache.Indexer

	PodEventHandler           cache.ResourceEventHandler
	NamespaceEventHandler     cache.ResourceEventHandler
	NetworkPolicyEventHandler cache.ResourceEventHandler
}

// internal structure to represent a network policy
type networkPolicyInfo struct {
	name        string
	namespace   string
	podSelector labels.Selector

	// set of pods matching network policy spec podselector label selector
	targetPods map[string]podInfo

	// whitelist ingress rules from the network policy spec
	ingressRules []ingressRule

	// whitelist egress rules from the network policy spec
	egressRules []egressRule

	// policy type "ingress" or "egress" or "both" as defined by PolicyType in the spec
	policyType string
}

// internal structure to represent Pod
type podInfo struct {
	ip        string
	name      string
	namespace string
	labels    map[string]string
}

// internal structure to represent NetworkPolicyIngressRule in the spec
type ingressRule struct {
	matchAllPorts  bool
	ports          []protocolAndPort
	namedPorts     []endPoints
	matchAllSource bool
	srcPods        []podInfo
	srcIPBlocks    [][]string
}

// internal structure to represent NetworkPolicyEgressRule in the spec
type egressRule struct {
	matchAllPorts        bool
	ports                []protocolAndPort
	namedPorts           []endPoints
	matchAllDestinations bool
	dstPods              []podInfo
	dstIPBlocks          [][]string
}

type protocolAndPort struct {
	protocol string
	port     string
}

type endPoints struct {
	ips []string
	protocolAndPort
}

type numericPort2eps map[string]*endPoints
type protocol2eps map[string]numericPort2eps
type namedPort2eps map[string]protocol2eps

// Run runs forever till we receive notification on stopCh
func (npc *NetworkPolicyController) Run(stopCh <-chan struct{}) {
	t := time.NewTicker(npc.syncPeriod)
	defer t.Stop()

	glog.Info("Starting network policy controller")

	// setup kube-router specific top level cutoms chains
	npc.ensureTopLevelChains()

	// Full syncs of the network policy controller take a lot of time and can only be processed one at a time,
	// therefore, we start it in it's own goroutine and request a sync through a single item channel
	glog.Info("Starting network policy controller full sync goroutine")
	go func(fullSyncRequest <-chan struct{}, stopCh <-chan struct{}) {
		for {
			// Add an additional non-blocking select to ensure that if the stopCh channel is closed it is handled first
			select {
			case <-stopCh:
				glog.Info("Shutting down network policies full sync goroutine")
				return
			default:
			}
			select {
			case <-stopCh:
				glog.Info("Shutting down network policies full sync goroutine")
				return
			case <-fullSyncRequest:
				glog.V(3).Info("Received request for a full sync, processing")
				npc.fullPolicySync() // fullPolicySync() is a blocking request here
			}
		}
	}(npc.fullSyncRequestChan, stopCh)

	// loop forever till notified to stop on stopCh
	for {
		glog.V(1).Info("Requesting periodic sync of iptables to reflect network policies")
		npc.RequestFullSync()
		select {
		case <-stopCh:
			glog.Infof("Shutting down network policies controller")
			return
		case <-t.C:
		}
	}
}

// RequestFullSync allows the request of a full network policy sync without blocking the callee
func (npc *NetworkPolicyController) RequestFullSync() {
	select {
	case npc.fullSyncRequestChan <- struct{}{}:
		glog.V(3).Info("Full sync request queue was empty so a full sync request was successfully sent")
	default: // Don't block if the buffered channel is full, return quickly so that we don't block callee execution
		glog.V(1).Info("Full sync request queue was full, skipping...")
	}
}

// Sync synchronizes iptables to desired state of network policies
func (npc *NetworkPolicyController) fullPolicySync() {

	var err error
	var networkPoliciesInfo []networkPolicyInfo
	npc.mu.Lock()
	defer npc.mu.Unlock()

	start := time.Now()
	syncVersion := strconv.FormatInt(start.UnixNano(), 10)
	defer func() {
		endTime := time.Since(start)
		glog.V(1).Infof("sync iptables took %v", endTime)
	}()

	glog.V(1).Infof("Starting sync of iptables with version: %s", syncVersion)

	// ensure kube-router specific top level chains and corresponding rules exist
	npc.ensureTopLevelChains()

	networkPoliciesInfo, err = npc.buildNetworkPoliciesInfo()
	if err != nil {
		glog.Errorf("Aborting sync. Failed to build network policies: %v", err.Error())
		return
	}

	activePolicyChains, activePolicyIPSets, err := npc.syncNetworkPolicyChains(networkPoliciesInfo, syncVersion)
	if err != nil {
		glog.Errorf("Aborting sync. Failed to sync network policy chains: %v" + err.Error())
		return
	}

	activePodFwChains, err := npc.syncPodFirewallChains(networkPoliciesInfo, syncVersion)
	if err != nil {
		glog.Errorf("Aborting sync. Failed to sync pod firewalls: %v", err.Error())
		return
	}

	err = cleanupStaleRules(activePolicyChains, activePodFwChains, activePolicyIPSets)
	if err != nil {
		glog.Errorf("Aborting sync. Failed to cleanup stale iptables rules: %v", err.Error())
		return
	}
}

// Creates custom chains KUBE-ROUTER-INPUT, KUBE-ROUTER-FORWARD, KUBE-ROUTER-OUTPUT
// and following rules in the filter table to jump from builtin chain to custom chain
// -A INPUT   -m comment --comment "kube-router netpol" -j KUBE-ROUTER-INPUT
// -A FORWARD -m comment --comment "kube-router netpol" -j KUBE-ROUTER-FORWARD
// -A OUTPUT  -m comment --comment "kube-router netpol" -j KUBE-ROUTER-OUTPUT
func (npc *NetworkPolicyController) ensureTopLevelChains() {

	iptablesCmdHandler, err := iptables.New()
	if err != nil {
		glog.Fatalf("Failed to initialize iptables executor due to %s", err.Error())
	}

	addUUIDForRuleSpec := func(chain string, ruleSpec *[]string) (string, error) {
		hash := sha256.Sum256([]byte(chain + strings.Join(*ruleSpec, "")))
		encoded := base32.StdEncoding.EncodeToString(hash[:])[:16]
		for idx, part := range *ruleSpec {
			if "--comment" == part {
				(*ruleSpec)[idx+1] = (*ruleSpec)[idx+1] + " - " + encoded
				return encoded, nil
			}
		}
		return "", fmt.Errorf("could not find a comment in the ruleSpec string given: %s", strings.Join(*ruleSpec, " "))
	}

	ensureRuleAtPosition := func(chain string, ruleSpec []string, uuid string, position int) {
		exists, err := iptablesCmdHandler.Exists("filter", chain, ruleSpec...)
		if err != nil {
			glog.Fatalf("Failed to verify rule exists in %s chain due to %s", chain, err.Error())
		}
		if !exists {
			err := iptablesCmdHandler.Insert("filter", chain, position, ruleSpec...)
			if err != nil {
				glog.Fatalf("Failed to run iptables command to insert in %s chain %s", chain, err.Error())
			}
			return
		}
		rules, err := iptablesCmdHandler.List("filter", chain)
		if err != nil {
			glog.Fatalf("failed to list rules in filter table %s chain due to %s", chain, err.Error())
		}

		var ruleNo, ruleIndexOffset int
		for i, rule := range rules {
			rule = strings.Replace(rule, "\"", "", 2) //removes quote from comment string
			if strings.HasPrefix(rule, "-P") || strings.HasPrefix(rule, "-N") {
				// if this chain has a default policy, then it will show as rule #1 from iptablesCmdHandler.List so we
				// need to account for this offset
				ruleIndexOffset++
				continue
			}
			if strings.Contains(rule, uuid) {
				// range uses a 0 index, but iptables uses a 1 index so we need to increase ruleNo by 1
				ruleNo = i + 1 - ruleIndexOffset
				break
			}
		}
		if ruleNo != position {
			err = iptablesCmdHandler.Insert("filter", chain, position, ruleSpec...)
			if err != nil {
				glog.Fatalf("Failed to run iptables command to insert in %s chain %s", chain, err.Error())
			}
			err = iptablesCmdHandler.Delete("filter", chain, strconv.Itoa(ruleNo+1))
			if err != nil {
				glog.Fatalf("Failed to delete incorrect rule in %s chain due to %s", chain, err.Error())
			}
		}
	}

	chains := map[string]string{"INPUT": kubeInputChainName, "FORWARD": kubeForwardChainName, "OUTPUT": kubeOutputChainName}

	for builtinChain, customChain := range chains {
		err = iptablesCmdHandler.NewChain("filter", customChain)
		if err != nil && err.(*iptables.Error).ExitStatus() != 1 {
			glog.Fatalf("Failed to run iptables command to create %s chain due to %s", customChain, err.Error())
		}
		args := []string{"-m", "comment", "--comment", "kube-router netpol", "-j", customChain}
		uuid, err := addUUIDForRuleSpec(builtinChain, &args)
		if err != nil {
			glog.Fatalf("Failed to get uuid for rule: %s", err.Error())
		}
		ensureRuleAtPosition(builtinChain, args, uuid, 1)
	}

	whitelistServiceVips := []string{"-m", "comment", "--comment", "allow traffic to cluster IP", "-d", npc.serviceClusterIPRange.String(), "-j", "RETURN"}
	uuid, err := addUUIDForRuleSpec(kubeInputChainName, &whitelistServiceVips)
	if err != nil {
		glog.Fatalf("Failed to get uuid for rule: %s", err.Error())
	}
	ensureRuleAtPosition(kubeInputChainName, whitelistServiceVips, uuid, 1)

	whitelistTCPNodeports := []string{"-p", "tcp", "-m", "comment", "--comment", "allow LOCAL TCP traffic to node ports", "-m", "addrtype", "--dst-type", "LOCAL",
		"-m", "multiport", "--dports", npc.serviceNodePortRange, "-j", "RETURN"}
	uuid, err = addUUIDForRuleSpec(kubeInputChainName, &whitelistTCPNodeports)
	if err != nil {
		glog.Fatalf("Failed to get uuid for rule: %s", err.Error())
	}
	ensureRuleAtPosition(kubeInputChainName, whitelistTCPNodeports, uuid, 2)

	whitelistUDPNodeports := []string{"-p", "udp", "-m", "comment", "--comment", "allow LOCAL UDP traffic to node ports", "-m", "addrtype", "--dst-type", "LOCAL",
		"-m", "multiport", "--dports", npc.serviceNodePortRange, "-j", "RETURN"}
	uuid, err = addUUIDForRuleSpec(kubeInputChainName, &whitelistUDPNodeports)
	if err != nil {
		glog.Fatalf("Failed to get uuid for rule: %s", err.Error())
	}
	ensureRuleAtPosition(kubeInputChainName, whitelistUDPNodeports, uuid, 3)

	for externalIPIndex, externalIPRange := range npc.serviceExternalIPRanges {
		whitelistServiceVips := []string{"-m", "comment", "--comment", "allow traffic to external IP range: " + externalIPRange.String(), "-d", externalIPRange.String(), "-j", "RETURN"}
		uuid, err = addUUIDForRuleSpec(kubeInputChainName, &whitelistServiceVips)
		if err != nil {
			glog.Fatalf("Failed to get uuid for rule: %s", err.Error())
		}
		ensureRuleAtPosition(kubeInputChainName, whitelistServiceVips, uuid, externalIPIndex+4)
	}

}

func cleanupStaleRules(activePolicyChains, activePodFwChains, activePolicyIPSets map[string]bool) error {

	cleanupPodFwChains := make([]string, 0)
	cleanupPolicyChains := make([]string, 0)
	cleanupPolicyIPSets := make([]*utils.Set, 0)

	// initialize tool sets for working with iptables and ipset
	iptablesCmdHandler, err := iptables.New()
	if err != nil {
		glog.Fatalf("failed to initialize iptables command executor due to %s", err.Error())
	}
	ipsets, err := utils.NewIPSet(false)
	if err != nil {
		glog.Fatalf("failed to create ipsets command executor due to %s", err.Error())
	}
	err = ipsets.Save()
	if err != nil {
		glog.Fatalf("failed to initialize ipsets command executor due to %s", err.Error())
	}

	// find iptables chains and ipsets that are no longer used by comparing current to the active maps we were passed
	chains, err := iptablesCmdHandler.ListChains("filter")
	if err != nil {
		return fmt.Errorf("Unable to list chains: %s", err)
	}
	for _, chain := range chains {
		if strings.HasPrefix(chain, kubeNetworkPolicyChainPrefix) {
			if _, ok := activePolicyChains[chain]; !ok {
				cleanupPolicyChains = append(cleanupPolicyChains, chain)
			}
		}
		if strings.HasPrefix(chain, kubePodFirewallChainPrefix) {
			if _, ok := activePodFwChains[chain]; !ok {
				cleanupPodFwChains = append(cleanupPodFwChains, chain)
			}
		}
	}
	for _, set := range ipsets.Sets {
		if strings.HasPrefix(set.Name, kubeSourceIPSetPrefix) ||
			strings.HasPrefix(set.Name, kubeDestinationIPSetPrefix) {
			if _, ok := activePolicyIPSets[set.Name]; !ok {
				cleanupPolicyIPSets = append(cleanupPolicyIPSets, set)
			}
		}
	}

	// remove stale iptables podFwChain references from the filter table chains
	for _, podFwChain := range cleanupPodFwChains {

		primaryChains := []string{kubeInputChainName, kubeForwardChainName, kubeOutputChainName}
		for _, egressChain := range primaryChains {
			forwardChainRules, err := iptablesCmdHandler.List("filter", egressChain)
			if err != nil {
				return fmt.Errorf("failed to list rules in filter table, %s podFwChain due to %s", egressChain, err.Error())
			}

			// TODO delete rule by spec, than rule number to avoid extra loop
			var realRuleNo int
			for i, rule := range forwardChainRules {
				if strings.Contains(rule, podFwChain) {
					err = iptablesCmdHandler.Delete("filter", egressChain, strconv.Itoa(i-realRuleNo))
					if err != nil {
						return fmt.Errorf("failed to delete rule: %s from the %s podFwChain of filter table due to %s", rule, egressChain, err.Error())
					}
					realRuleNo++
				}
			}
		}
	}

	// cleanup pod firewall chain
	for _, chain := range cleanupPodFwChains {
		glog.V(2).Infof("Found pod fw chain to cleanup: %s", chain)
		err = iptablesCmdHandler.ClearChain("filter", chain)
		if err != nil {
			return fmt.Errorf("Failed to flush the rules in chain %s due to %s", chain, err.Error())
		}
		err = iptablesCmdHandler.DeleteChain("filter", chain)
		if err != nil {
			return fmt.Errorf("Failed to delete the chain %s due to %s", chain, err.Error())
		}
		glog.V(2).Infof("Deleted pod specific firewall chain: %s from the filter table", chain)
	}

	// cleanup network policy chains
	for _, policyChain := range cleanupPolicyChains {
		glog.V(2).Infof("Found policy chain to cleanup %s", policyChain)

		// first clean up any references from active pod firewall chains
		for podFwChain := range activePodFwChains {
			podFwChainRules, err := iptablesCmdHandler.List("filter", podFwChain)
			if err != nil {
				return fmt.Errorf("Unable to list rules from the chain %s: %s", podFwChain, err)
			}
			for i, rule := range podFwChainRules {
				if strings.Contains(rule, policyChain) {
					err = iptablesCmdHandler.Delete("filter", podFwChain, strconv.Itoa(i))
					if err != nil {
						return fmt.Errorf("Failed to delete rule %s from the chain %s", rule, podFwChain)
					}
					break
				}
			}
		}

		// now that all stale and active references to the network policy chain have been removed, delete the chain
		err = iptablesCmdHandler.ClearChain("filter", policyChain)
		if err != nil {
			return fmt.Errorf("Failed to flush the rules in chain %s due to  %s", policyChain, err)
		}
		err = iptablesCmdHandler.DeleteChain("filter", policyChain)
		if err != nil {
			return fmt.Errorf("Failed to flush the rules in chain %s due to %s", policyChain, err)
		}
		glog.V(2).Infof("Deleted network policy chain: %s from the filter table", policyChain)
	}

	// cleanup network policy ipsets
	for _, set := range cleanupPolicyIPSets {
		err = set.Destroy()
		if err != nil {
			return fmt.Errorf("Failed to delete ipset %s due to %s", set.Name, err)
		}
	}
	return nil
}

// Cleanup cleanup configurations done
func (npc *NetworkPolicyController) Cleanup() {

	glog.Info("Cleaning up iptables configuration permanently done by kube-router")

	iptablesCmdHandler, err := iptables.New()
	if err != nil {
		glog.Errorf("Failed to initialize iptables executor: %s", err.Error())
	}

	// delete jump rules in FORWARD chain to pod specific firewall chain
	forwardChainRules, err := iptablesCmdHandler.List("filter", kubeForwardChainName)
	if err != nil {
		glog.Errorf("Failed to delete iptables rules as part of cleanup")
		return
	}

	// TODO: need a better way to delete rule with out using number
	var realRuleNo int
	for i, rule := range forwardChainRules {
		if strings.Contains(rule, kubePodFirewallChainPrefix) {
			err = iptablesCmdHandler.Delete("filter", kubeForwardChainName, strconv.Itoa(i-realRuleNo))
			if err != nil {
				glog.Errorf("Failed to delete iptables rule as part of cleanup: %s", err)
			}
			realRuleNo++
		}
	}

	// delete jump rules in OUTPUT chain to pod specific firewall chain
	forwardChainRules, err = iptablesCmdHandler.List("filter", kubeOutputChainName)
	if err != nil {
		glog.Errorf("Failed to delete iptables rules as part of cleanup")
		return
	}

	// TODO: need a better way to delete rule with out using number
	realRuleNo = 0
	for i, rule := range forwardChainRules {
		if strings.Contains(rule, kubePodFirewallChainPrefix) {
			err = iptablesCmdHandler.Delete("filter", kubeOutputChainName, strconv.Itoa(i-realRuleNo))
			if err != nil {
				glog.Errorf("Failed to delete iptables rule as part of cleanup: %s", err)
			}
			realRuleNo++
		}
	}

	// flush and delete pod specific firewall chain
	chains, err := iptablesCmdHandler.ListChains("filter")
	if err != nil {
		glog.Errorf("Unable to list chains: %s", err)
		return
	}
	for _, chain := range chains {
		if strings.HasPrefix(chain, kubePodFirewallChainPrefix) {
			err = iptablesCmdHandler.ClearChain("filter", chain)
			if err != nil {
				glog.Errorf("Failed to cleanup iptables rules: " + err.Error())
				return
			}
			err = iptablesCmdHandler.DeleteChain("filter", chain)
			if err != nil {
				glog.Errorf("Failed to cleanup iptables rules: " + err.Error())
				return
			}
		}
	}

	// flush and delete per network policy specific chain
	chains, err = iptablesCmdHandler.ListChains("filter")
	if err != nil {
		glog.Errorf("Unable to list chains: %s", err)
		return
	}
	for _, chain := range chains {
		if strings.HasPrefix(chain, kubeNetworkPolicyChainPrefix) {
			err = iptablesCmdHandler.ClearChain("filter", chain)
			if err != nil {
				glog.Errorf("Failed to cleanup iptables rules: " + err.Error())
				return
			}
			err = iptablesCmdHandler.DeleteChain("filter", chain)
			if err != nil {
				glog.Errorf("Failed to cleanup iptables rules: " + err.Error())
				return
			}
		}
	}

	// delete all ipsets
	ipset, err := utils.NewIPSet(false)
	if err != nil {
		glog.Errorf("Failed to clean up ipsets: " + err.Error())
	}
	err = ipset.Save()
	if err != nil {
		glog.Errorf("Failed to clean up ipsets: " + err.Error())
	}
	err = ipset.DestroyAllWithin()
	if err != nil {
		glog.Errorf("Failed to clean up ipsets: " + err.Error())
	}
	glog.Infof("Successfully cleaned the iptables configuration done by kube-router")
}

// NewNetworkPolicyController returns new NetworkPolicyController object
func NewNetworkPolicyController(clientset kubernetes.Interface,
	config *config.Node, podInformer cache.SharedIndexInformer,
	npInformer cache.SharedIndexInformer, nsInformer cache.SharedIndexInformer) (*NetworkPolicyController, error) {
	npc := NetworkPolicyController{}

	// Creating a single-item buffered channel to ensure that we only keep a single full sync request at a time,
	// additional requests would be pointless to queue since after the first one was processed the system would already
	// be up to date with all of the policy changes from any enqueued request after that
	npc.fullSyncRequestChan = make(chan struct{}, 1)

	npc.serviceClusterIPRange = *config.AgentConfig.ServiceCIDR
	npc.serviceNodePortRange = strings.ReplaceAll(config.AgentConfig.ServiceNodePortRange.String(), "-", ":")
	npc.syncPeriod = defaultSyncPeriod

	node, err := utils.GetNodeObject(clientset, config.AgentConfig.NodeName)
	if err != nil {
		return nil, err
	}

	npc.nodeHostName = node.Name

	nodeIP, err := utils.GetNodeIP(node)
	if err != nil {
		return nil, err
	}
	npc.nodeIP = nodeIP

	ipset, err := utils.NewIPSet(false)
	if err != nil {
		return nil, err
	}
	err = ipset.Save()
	if err != nil {
		return nil, err
	}
	npc.ipSetHandler = ipset

	npc.podLister = podInformer.GetIndexer()
	npc.PodEventHandler = npc.newPodEventHandler()

	npc.nsLister = nsInformer.GetIndexer()
	npc.NamespaceEventHandler = npc.newNamespaceEventHandler()

	npc.npLister = npInformer.GetIndexer()
	npc.NetworkPolicyEventHandler = npc.newNetworkPolicyEventHandler()

	return &npc, nil
}
