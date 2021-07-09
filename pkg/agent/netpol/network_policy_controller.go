// Apache License v2.0 (copyright Cloud Native Labs & Rancher Labs)
// - modified from https://github.com/cloudnativelabs/kube-router/blob/73b1b03b32c5755b240f6c077bb097abe3888314/pkg/controllers/netpol/network_policy_controller.go

// +build !windows

package netpol

import (
	"bytes"
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
	"k8s.io/klog/v2"
)

const (
	kubePodFirewallChainPrefix   = "KUBE-POD-FW-"
	kubeNetworkPolicyChainPrefix = "KUBE-NWPLCY-"
	kubeSourceIPSetPrefix        = "KUBE-SRC-"
	kubeDestinationIPSetPrefix   = "KUBE-DST-"
	kubeInputChainName           = "KUBE-ROUTER-INPUT"
	kubeForwardChainName         = "KUBE-ROUTER-FORWARD"
	kubeOutputChainName          = "KUBE-ROUTER-OUTPUT"
	kubeDefaultNetpolChain       = "KUBE-NWPLCY-DEFAULT"
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

// NetworkPolicyController struct to hold information required by NetworkPolicyController
type NetworkPolicyController struct {
	nodeIP                  net.IP
	nodeHostName            string
	serviceClusterIPRange   net.IPNet
	serviceExternalIPRanges []net.IPNet
	serviceNodePortRange    string
	mu                      sync.Mutex
	syncPeriod              time.Duration
	fullSyncRequestChan     chan struct{}
	ipsetMutex              *sync.Mutex

	ipSetHandler *utils.IPSet

	podLister cache.Indexer
	npLister  cache.Indexer
	nsLister  cache.Indexer

	PodEventHandler           cache.ResourceEventHandler
	NamespaceEventHandler     cache.ResourceEventHandler
	NetworkPolicyEventHandler cache.ResourceEventHandler

	filterTableRules bytes.Buffer
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
	endport  string
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

	klog.Info("Starting network policy controller")

	// setup kube-router specific top level custom chains (KUBE-ROUTER-INPUT, KUBE-ROUTER-FORWARD, KUBE-ROUTER-OUTPUT)
	npc.ensureTopLevelChains()

	// setup default network policy chain that is applied to traffic from/to the pods that does not match any network policy
	npc.ensureDefaultNetworkPolicyChain()

	// Full syncs of the network policy controller take a lot of time and can only be processed one at a time,
	// therefore, we start it in it's own goroutine and request a sync through a single item channel
	klog.Info("Starting network policy controller full sync goroutine")
	go func(fullSyncRequest <-chan struct{}, stopCh <-chan struct{}) {
		for {
			// Add an additional non-blocking select to ensure that if the stopCh channel is closed it is handled first
			select {
			case <-stopCh:
				klog.Info("Shutting down network policies full sync goroutine")
				return
			default:
			}
			select {
			case <-stopCh:
				klog.Info("Shutting down network policies full sync goroutine")
				return
			case <-fullSyncRequest:
				klog.V(3).Info("Received request for a full sync, processing")
				npc.fullPolicySync() // fullPolicySync() is a blocking request here
			}
		}
	}(npc.fullSyncRequestChan, stopCh)

	// loop forever till notified to stop on stopCh
	for {
		klog.V(1).Info("Requesting periodic sync of iptables to reflect network policies")
		npc.RequestFullSync()
		select {
		case <-stopCh:
			klog.Infof("Shutting down network policies controller")
			return
		case <-t.C:
		}
	}
}

// RequestFullSync allows the request of a full network policy sync without blocking the callee
func (npc *NetworkPolicyController) RequestFullSync() {
	select {
	case npc.fullSyncRequestChan <- struct{}{}:
		klog.V(3).Info("Full sync request queue was empty so a full sync request was successfully sent")
	default: // Don't block if the buffered channel is full, return quickly so that we don't block callee execution
		klog.V(1).Info("Full sync request queue was full, skipping...")
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
		klog.V(1).Infof("sync iptables took %v", endTime)
	}()

	klog.V(1).Infof("Starting sync of iptables with version: %s", syncVersion)

	// ensure kube-router specific top level chains and corresponding rules exist
	npc.ensureTopLevelChains()

	// ensure default network policy chain that is applied to traffic from/to the pods that does not match any network policy
	npc.ensureDefaultNetworkPolicyChain()

	networkPoliciesInfo, err = npc.buildNetworkPoliciesInfo()
	if err != nil {
		klog.Errorf("Aborting sync. Failed to build network policies: %v", err.Error())
		return
	}

	npc.filterTableRules.Reset()
	if err := utils.SaveInto("filter", &npc.filterTableRules); err != nil {
		klog.Errorf("Aborting sync. Failed to run iptables-save: %v" + err.Error())
		return
	}

	activePolicyChains, activePolicyIPSets, err := npc.syncNetworkPolicyChains(networkPoliciesInfo, syncVersion)
	if err != nil {
		klog.Errorf("Aborting sync. Failed to sync network policy chains: %v" + err.Error())
		return
	}

	activePodFwChains, err := npc.syncPodFirewallChains(networkPoliciesInfo, syncVersion)
	if err != nil {
		klog.Errorf("Aborting sync. Failed to sync pod firewalls: %v", err.Error())
		return
	}

	err = npc.cleanupStaleRules(activePolicyChains, activePodFwChains)
	if err != nil {
		klog.Errorf("Aborting sync. Failed to cleanup stale iptables rules: %v", err.Error())
		return
	}

	if err := utils.Restore("filter", npc.filterTableRules.Bytes()); err != nil {
		klog.Errorf("Aborting sync. Failed to run iptables-restore: %v\n%s", err.Error(), npc.filterTableRules.String())
		return
	}

	err = npc.cleanupStaleIPSets(activePolicyIPSets)
	if err != nil {
		klog.Errorf("Failed to cleanup stale ipsets: %v", err.Error())
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
		klog.Fatalf("Failed to initialize iptables executor due to %s", err.Error())
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
			klog.Fatalf("Failed to verify rule exists in %s chain due to %s", chain, err.Error())
		}
		if !exists {
			err := iptablesCmdHandler.Insert("filter", chain, position, ruleSpec...)
			if err != nil {
				klog.Fatalf("Failed to run iptables command to insert in %s chain %s", chain, err.Error())
			}
			return
		}
		rules, err := iptablesCmdHandler.List("filter", chain)
		if err != nil {
			klog.Fatalf("failed to list rules in filter table %s chain due to %s", chain, err.Error())
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
				klog.Fatalf("Failed to run iptables command to insert in %s chain %s", chain, err.Error())
			}
			err = iptablesCmdHandler.Delete("filter", chain, strconv.Itoa(ruleNo+1))
			if err != nil {
				klog.Fatalf("Failed to delete incorrect rule in %s chain due to %s", chain, err.Error())
			}
		}
	}

	chains := map[string]string{"INPUT": kubeInputChainName, "FORWARD": kubeForwardChainName, "OUTPUT": kubeOutputChainName}

	for builtinChain, customChain := range chains {
		err = iptablesCmdHandler.NewChain("filter", customChain)
		if err != nil && err.(*iptables.Error).ExitStatus() != 1 {
			klog.Fatalf("Failed to run iptables command to create %s chain due to %s", customChain, err.Error())
		}
		args := []string{"-m", "comment", "--comment", "kube-router netpol", "-j", customChain}
		uuid, err := addUUIDForRuleSpec(builtinChain, &args)
		if err != nil {
			klog.Fatalf("Failed to get uuid for rule: %s", err.Error())
		}
		ensureRuleAtPosition(builtinChain, args, uuid, 1)
	}

	whitelistServiceVips := []string{"-m", "comment", "--comment", "allow traffic to cluster IP", "-d", npc.serviceClusterIPRange.String(), "-j", "RETURN"}
	uuid, err := addUUIDForRuleSpec(kubeInputChainName, &whitelistServiceVips)
	if err != nil {
		klog.Fatalf("Failed to get uuid for rule: %s", err.Error())
	}
	ensureRuleAtPosition(kubeInputChainName, whitelistServiceVips, uuid, 1)

	whitelistTCPNodeports := []string{"-p", "tcp", "-m", "comment", "--comment", "allow LOCAL TCP traffic to node ports", "-m", "addrtype", "--dst-type", "LOCAL",
		"-m", "multiport", "--dports", npc.serviceNodePortRange, "-j", "RETURN"}
	uuid, err = addUUIDForRuleSpec(kubeInputChainName, &whitelistTCPNodeports)
	if err != nil {
		klog.Fatalf("Failed to get uuid for rule: %s", err.Error())
	}
	ensureRuleAtPosition(kubeInputChainName, whitelistTCPNodeports, uuid, 2)

	whitelistUDPNodeports := []string{"-p", "udp", "-m", "comment", "--comment", "allow LOCAL UDP traffic to node ports", "-m", "addrtype", "--dst-type", "LOCAL",
		"-m", "multiport", "--dports", npc.serviceNodePortRange, "-j", "RETURN"}
	uuid, err = addUUIDForRuleSpec(kubeInputChainName, &whitelistUDPNodeports)
	if err != nil {
		klog.Fatalf("Failed to get uuid for rule: %s", err.Error())
	}
	ensureRuleAtPosition(kubeInputChainName, whitelistUDPNodeports, uuid, 3)

	for externalIPIndex, externalIPRange := range npc.serviceExternalIPRanges {
		whitelistServiceVips := []string{"-m", "comment", "--comment", "allow traffic to external IP range: " + externalIPRange.String(), "-d", externalIPRange.String(), "-j", "RETURN"}
		uuid, err = addUUIDForRuleSpec(kubeInputChainName, &whitelistServiceVips)
		if err != nil {
			klog.Fatalf("Failed to get uuid for rule: %s", err.Error())
		}
		ensureRuleAtPosition(kubeInputChainName, whitelistServiceVips, uuid, externalIPIndex+4)
	}

	// for the traffic to/from the local pod's let network policy controller be
	// authoritative entity to ACCEPT the traffic if it complies to network policies
	for _, chain := range chains {
		comment := "rule to explicitly ACCEPT traffic that comply to network policies"
		args := []string{"-m", "comment", "--comment", comment, "-m", "mark", "--mark", "0x20000/0x20000", "-j", "ACCEPT"}
		err = iptablesCmdHandler.AppendUnique("filter", chain, args...)
		if err != nil {
			klog.Fatalf("Failed to run iptables command: %s", err.Error())
		}
	}
}

// Creates custom chains KUBE-NWPLCY-DEFAULT
func (npc *NetworkPolicyController) ensureDefaultNetworkPolicyChain() {

	iptablesCmdHandler, err := iptables.New()
	if err != nil {
		klog.Fatalf("Failed to initialize iptables executor due to %s", err.Error())
	}

	markArgs := make([]string, 0)
	markComment := "rule to mark traffic matching a network policy"
	markArgs = append(markArgs, "-j", "MARK", "-m", "comment", "--comment", markComment, "--set-xmark", "0x10000/0x10000")

	err = iptablesCmdHandler.NewChain("filter", kubeDefaultNetpolChain)
	if err != nil && err.(*iptables.Error).ExitStatus() != 1 {
		klog.Fatalf("Failed to run iptables command to create %s chain due to %s", kubeDefaultNetpolChain, err.Error())
	}
	err = iptablesCmdHandler.AppendUnique("filter", kubeDefaultNetpolChain, markArgs...)
	if err != nil {
		klog.Fatalf("Failed to run iptables command: %s", err.Error())
	}
}

func (npc *NetworkPolicyController) cleanupStaleRules(activePolicyChains, activePodFwChains map[string]bool) error {

	cleanupPodFwChains := make([]string, 0)
	cleanupPolicyChains := make([]string, 0)

	// initialize tool sets for working with iptables and ipset
	iptablesCmdHandler, err := iptables.New()
	if err != nil {
		return fmt.Errorf("failed to initialize iptables command executor due to %s", err.Error())
	}

	// find iptables chains and ipsets that are no longer used by comparing current to the active maps we were passed
	chains, err := iptablesCmdHandler.ListChains("filter")
	if err != nil {
		return fmt.Errorf("unable to list chains: %s", err)
	}
	for _, chain := range chains {
		if strings.HasPrefix(chain, kubeNetworkPolicyChainPrefix) {
			if chain == kubeDefaultNetpolChain {
				continue
			}
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

	var newChains, newRules, desiredFilterTable bytes.Buffer
	rules := strings.Split(npc.filterTableRules.String(), "\n")
	if len(rules) > 0 && rules[len(rules)-1] == "" {
		rules = rules[:len(rules)-1]
	}
	for _, rule := range rules {
		skipRule := false
		for _, podFWChainName := range cleanupPodFwChains {
			if strings.Contains(rule, podFWChainName) {
				skipRule = true
				break
			}
		}
		for _, policyChainName := range cleanupPolicyChains {
			if strings.Contains(rule, policyChainName) {
				skipRule = true
				break
			}
		}
		if strings.Contains(rule, "COMMIT") || strings.HasPrefix(rule, "# ") {
			skipRule = true
		}
		if skipRule {
			continue
		}
		if strings.HasPrefix(rule, ":") {
			newChains.WriteString(rule + " - [0:0]\n")
		}
		if strings.HasPrefix(rule, "-") {
			newRules.WriteString(rule + "\n")
		}
	}
	desiredFilterTable.WriteString("*filter" + "\n")
	desiredFilterTable.Write(newChains.Bytes())
	desiredFilterTable.Write(newRules.Bytes())
	desiredFilterTable.WriteString("COMMIT" + "\n")
	npc.filterTableRules = desiredFilterTable

	return nil
}

func (npc *NetworkPolicyController) cleanupStaleIPSets(activePolicyIPSets map[string]bool) error {
	cleanupPolicyIPSets := make([]*utils.Set, 0)
	ipsets, err := utils.NewIPSet(false)
	if err != nil {
		return fmt.Errorf("failed to create ipsets command executor due to %s", err.Error())
	}
	err = ipsets.Save()
	if err != nil {
		klog.Fatalf("failed to initialize ipsets command executor due to %s", err.Error())
	}
	for _, set := range ipsets.Sets {
		if strings.HasPrefix(set.Name, kubeSourceIPSetPrefix) ||
			strings.HasPrefix(set.Name, kubeDestinationIPSetPrefix) {
			if _, ok := activePolicyIPSets[set.Name]; !ok {
				cleanupPolicyIPSets = append(cleanupPolicyIPSets, set)
			}
		}
	}
	// cleanup network policy ipsets
	for _, set := range cleanupPolicyIPSets {
		err = set.Destroy()
		if err != nil {
			return fmt.Errorf("failed to delete ipset %s due to %s", set.Name, err)
		}
	}
	return nil
}

// Cleanup cleanup configurations done
func (npc *NetworkPolicyController) Cleanup() {

	klog.Info("Cleaning up iptables configuration permanently done by kube-router")

	iptablesCmdHandler, err := iptables.New()
	if err != nil {
		klog.Errorf("Failed to initialize iptables executor: %s", err.Error())
		return
	}

	// delete jump rules in FORWARD chain to pod specific firewall chain
	forwardChainRules, err := iptablesCmdHandler.List("filter", kubeForwardChainName)
	if err != nil {
		klog.Errorf("Failed to delete iptables rules as part of cleanup")
		return
	}

	// TODO: need a better way to delete rule with out using number
	var realRuleNo int
	for i, rule := range forwardChainRules {
		if strings.Contains(rule, kubePodFirewallChainPrefix) {
			err = iptablesCmdHandler.Delete("filter", kubeForwardChainName, strconv.Itoa(i-realRuleNo))
			if err != nil {
				klog.Errorf("Failed to delete iptables rule as part of cleanup: %s", err)
			}
			realRuleNo++
		}
	}

	// delete jump rules in OUTPUT chain to pod specific firewall chain
	forwardChainRules, err = iptablesCmdHandler.List("filter", kubeOutputChainName)
	if err != nil {
		klog.Errorf("Failed to delete iptables rules as part of cleanup")
		return
	}

	// TODO: need a better way to delete rule with out using number
	realRuleNo = 0
	for i, rule := range forwardChainRules {
		if strings.Contains(rule, kubePodFirewallChainPrefix) {
			err = iptablesCmdHandler.Delete("filter", kubeOutputChainName, strconv.Itoa(i-realRuleNo))
			if err != nil {
				klog.Errorf("Failed to delete iptables rule as part of cleanup: %s", err)
			}
			realRuleNo++
		}
	}

	// flush and delete pod specific firewall chain
	chains, err := iptablesCmdHandler.ListChains("filter")
	if err != nil {
		klog.Errorf("Unable to list chains: %s", err)
		return
	}
	for _, chain := range chains {
		if strings.HasPrefix(chain, kubePodFirewallChainPrefix) {
			err = iptablesCmdHandler.ClearChain("filter", chain)
			if err != nil {
				klog.Errorf("Failed to cleanup iptables rules: " + err.Error())
				return
			}
			err = iptablesCmdHandler.DeleteChain("filter", chain)
			if err != nil {
				klog.Errorf("Failed to cleanup iptables rules: " + err.Error())
				return
			}
		}
	}

	// flush and delete per network policy specific chain
	chains, err = iptablesCmdHandler.ListChains("filter")
	if err != nil {
		klog.Errorf("Unable to list chains: %s", err)
		return
	}
	for _, chain := range chains {
		if strings.HasPrefix(chain, kubeNetworkPolicyChainPrefix) {
			err = iptablesCmdHandler.ClearChain("filter", chain)
			if err != nil {
				klog.Errorf("Failed to cleanup iptables rules: " + err.Error())
				return
			}
			err = iptablesCmdHandler.DeleteChain("filter", chain)
			if err != nil {
				klog.Errorf("Failed to cleanup iptables rules: " + err.Error())
				return
			}
		}
	}

	// delete all ipsets
	klog.V(1).Infof("Attempting to attain ipset mutex lock")
	npc.ipsetMutex.Lock()
	klog.V(1).Infof("Attained ipset mutex lock, continuing...")
	defer func() {
		npc.ipsetMutex.Unlock()
		klog.V(1).Infof("Returned ipset mutex lock")
	}()
	ipset, err := utils.NewIPSet(false)
	if err != nil {
		klog.Errorf("Failed to clean up ipsets: " + err.Error())
		return
	}
	err = ipset.Save()
	if err != nil {
		klog.Errorf("Failed to clean up ipsets: " + err.Error())
	}
	err = ipset.DestroyAllWithin()
	if err != nil {
		klog.Errorf("Failed to clean up ipsets: " + err.Error())
	}
	klog.Infof("Successfully cleaned the iptables configuration done by kube-router")
}

// NewNetworkPolicyController returns new NetworkPolicyController object
func NewNetworkPolicyController(clientset kubernetes.Interface,
	config *config.Node, podInformer cache.SharedIndexInformer,
	npInformer cache.SharedIndexInformer, nsInformer cache.SharedIndexInformer, ipsetMutex *sync.Mutex) (*NetworkPolicyController, error) {
	npc := NetworkPolicyController{ipsetMutex: ipsetMutex}

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

	nodeIP, err := utils.GetNodeIP(node)
	if err != nil {
		return nil, err
	}
	npc.nodeIP = nodeIP

	npc.podLister = podInformer.GetIndexer()
	npc.PodEventHandler = npc.newPodEventHandler()

	npc.nsLister = nsInformer.GetIndexer()
	npc.NamespaceEventHandler = npc.newNamespaceEventHandler()

	npc.npLister = npInformer.GetIndexer()
	npc.NetworkPolicyEventHandler = npc.newNetworkPolicyEventHandler()

	return &npc, nil
}
