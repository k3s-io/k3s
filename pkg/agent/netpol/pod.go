// Apache License v2.0 (copyright Cloud Native Labs & Rancher Labs)
// - modified from https://github.com/cloudnativelabs/kube-router/blob/73b1b03b32c5755b240f6c077bb097abe3888314/pkg/controllers/netpol/pod.go

// +build !windows

package netpol

import (
	"crypto/sha256"
	"encoding/base32"
	"strings"

	api "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (npc *NetworkPolicyController) newPodEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if podObj, ok := obj.(*api.Pod); ok {
				// If the pod isn't yet actionable there is no action to take here anyway, so skip it. When it becomes
				// actionable, we'll get an update below.
				if isNetPolActionable(podObj) {
					npc.OnPodUpdate(obj)
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			var newPodObj, oldPodObj *api.Pod
			var ok bool

			// If either of these objects are not pods, quit now
			if newPodObj, ok = newObj.(*api.Pod); !ok {
				return
			}
			if oldPodObj, ok = oldObj.(*api.Pod); !ok {
				return
			}

			// We don't check isNetPolActionable here, because if it is transitioning in or out of the actionable state
			// we want to run the full sync so that it can be added or removed from the existing network policy of the host
			// For the network policies, we are only interested in some changes, most pod changes aren't relevant to network policy
			if isPodUpdateNetPolRelevant(oldPodObj, newPodObj) {
				npc.OnPodUpdate(newObj)
			}
		},
		DeleteFunc: func(obj interface{}) {
			npc.handlePodDelete(obj)
		},
	}
}

// OnPodUpdate handles updates to pods from the Kubernetes api server
func (npc *NetworkPolicyController) OnPodUpdate(obj interface{}) {
	pod := obj.(*api.Pod)
	klog.V(2).Infof("Received update to pod: %s/%s", pod.Namespace, pod.Name)

	npc.RequestFullSync()
}

func (npc *NetworkPolicyController) handlePodDelete(obj interface{}) {
	pod, ok := obj.(*api.Pod)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("unexpected object type: %v", obj)
			return
		}
		if pod, ok = tombstone.Obj.(*api.Pod); !ok {
			klog.Errorf("unexpected object type: %v", obj)
			return
		}
	}
	klog.V(2).Infof("Received pod: %s/%s delete event", pod.Namespace, pod.Name)

	npc.RequestFullSync()
}

func (npc *NetworkPolicyController) syncPodFirewallChains(networkPoliciesInfo []networkPolicyInfo, version string) (map[string]bool, error) {

	activePodFwChains := make(map[string]bool)

	dropUnmarkedTrafficRules := func(podName, podNamespace, podFwChainName string) error {
		// add rule to log the packets that will be dropped due to network policy enforcement
		comment := "\"rule to log dropped traffic POD name:" + podName + " namespace: " + podNamespace + "\""
		args := []string{"-A", podFwChainName, "-m", "comment", "--comment", comment, "-m", "mark", "!", "--mark", "0x10000/0x10000", "-j", "NFLOG", "--nflog-group", "100", "-m", "limit", "--limit", "10/minute", "--limit-burst", "10", "\n"}
		// This used to be AppendUnique when we were using iptables directly, this checks to make sure we didn't drop unmarked for this chain already
		if strings.Contains(npc.filterTableRules.String(), strings.Join(args, " ")) {
			return nil
		}
		npc.filterTableRules.WriteString(strings.Join(args, " "))

		// add rule to DROP if no applicable network policy permits the traffic
		comment = "\"rule to REJECT traffic destined for POD name:" + podName + " namespace: " + podNamespace + "\""
		args = []string{"-A", podFwChainName, "-m", "comment", "--comment", comment, "-m", "mark", "!", "--mark", "0x10000/0x10000", "-j", "REJECT", "\n"}
		npc.filterTableRules.WriteString(strings.Join(args, " "))

		// reset mark to let traffic pass through rest of the chains
		args = []string{"-A", podFwChainName, "-j", "MARK", "--set-mark", "0/0x10000", "\n"}
		npc.filterTableRules.WriteString(strings.Join(args, " "))

		return nil
	}

	// loop through the pods running on the node
	allLocalPods, err := npc.getLocalPods(npc.nodeIP.String())
	if err != nil {
		return nil, err
	}
	for _, pod := range *allLocalPods {

		// ensure pod specific firewall chain exist for all the pods that need ingress firewall
		podFwChainName := podFirewallChainName(pod.namespace, pod.name, version)
		npc.filterTableRules.WriteString(":" + podFwChainName + "\n")

		activePodFwChains[podFwChainName] = true

		// setup rules to run through applicable ingress/egress network policies for the pod
		npc.setupPodNetpolRules(&pod, podFwChainName, networkPoliciesInfo, version)

		// setup rules to intercept inbound traffic to the pods
		npc.interceptPodInboundTraffic(&pod, podFwChainName)

		// setup rules to intercept inbound traffic to the pods
		npc.interceptPodOutboundTraffic(&pod, podFwChainName)

		err = dropUnmarkedTrafficRules(pod.name, pod.namespace, podFwChainName)
		if err != nil {
			return nil, err
		}

		// set mark to indicate traffic from/to the pod passed network policies.
		// Mark will be checked to explicitly ACCEPT the traffic
		comment := "\"set mark to ACCEPT traffic that comply to network policies\""
		args := []string{"-A", podFwChainName, "-m", "comment", "--comment", comment, "-j", "MARK", "--set-mark", "0x20000/0x20000", "\n"}
		npc.filterTableRules.WriteString(strings.Join(args, " "))
	}

	return activePodFwChains, nil
}

// setup rules to jump to applicable network policy chains for the traffic from/to the pod
func (npc *NetworkPolicyController) setupPodNetpolRules(pod *podInfo, podFwChainName string, networkPoliciesInfo []networkPolicyInfo, version string) {

	hasIngressPolicy := false
	hasEgressPolicy := false

	// add entries in pod firewall to run through applicable network policies
	for _, policy := range networkPoliciesInfo {
		if _, ok := policy.targetPods[pod.ip]; !ok {
			continue
		}
		comment := "\"run through nw policy " + policy.name + "\""
		policyChainName := networkPolicyChainName(policy.namespace, policy.name, version)
		var args []string
		switch policy.policyType {
		case "both":
			hasIngressPolicy = true
			hasEgressPolicy = true
			args = []string{"-I", podFwChainName, "1", "-m", "comment", "--comment", comment, "-j", policyChainName, "\n"}
		case "ingress":
			hasIngressPolicy = true
			args = []string{"-I", podFwChainName, "1", "-d", pod.ip, "-m", "comment", "--comment", comment, "-j", policyChainName, "\n"}
		case "egress":
			hasEgressPolicy = true
			args = []string{"-I", podFwChainName, "1", "-s", pod.ip, "-m", "comment", "--comment", comment, "-j", policyChainName, "\n"}
		}
		npc.filterTableRules.WriteString(strings.Join(args, " "))
	}

	// if pod does not have any network policy which applies rules for pod's ingress traffic
	// then apply default network policy
	if !hasIngressPolicy {
		comment := "\"run through default ingress network policy  chain\""
		args := []string{"-I", podFwChainName, "1", "-d", pod.ip, "-m", "comment", "--comment", comment, "-j", kubeDefaultNetpolChain, "\n"}
		npc.filterTableRules.WriteString(strings.Join(args, " "))
	}

	// if pod does not have any network policy which applies rules for pod's egress traffic
	// then apply default network policy
	if !hasEgressPolicy {
		comment := "\"run through default egress network policy  chain\""
		args := []string{"-I", podFwChainName, "1", "-s", pod.ip, "-m", "comment", "--comment", comment, "-j", kubeDefaultNetpolChain, "\n"}
		npc.filterTableRules.WriteString(strings.Join(args, " "))
	}

	comment := "\"rule to permit the traffic traffic to pods when source is the pod's local node\""
	args := []string{"-I", podFwChainName, "1", "-m", "comment", "--comment", comment, "-m", "addrtype", "--src-type", "LOCAL", "-d", pod.ip, "-j", "ACCEPT", "\n"}
	npc.filterTableRules.WriteString(strings.Join(args, " "))

	// ensure statefull firewall that permits RELATED,ESTABLISHED traffic from/to the pod
	comment = "\"rule for stateful firewall for pod\""
	args = []string{"-I", podFwChainName, "1", "-m", "comment", "--comment", comment, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT", "\n"}
	npc.filterTableRules.WriteString(strings.Join(args, " "))

}

func (npc *NetworkPolicyController) interceptPodInboundTraffic(pod *podInfo, podFwChainName string) {
	// ensure there is rule in filter table and FORWARD chain to jump to pod specific firewall chain
	// this rule applies to the traffic getting routed (coming for other node pods)
	comment := "\"rule to jump traffic destined to POD name:" + pod.name + " namespace: " + pod.namespace +
		" to chain " + podFwChainName + "\""
	args := []string{"-I", kubeForwardChainName, "1", "-m", "comment", "--comment", comment, "-d", pod.ip, "-j", podFwChainName + "\n"}
	npc.filterTableRules.WriteString(strings.Join(args, " "))

	// ensure there is rule in filter table and OUTPUT chain to jump to pod specific firewall chain
	// this rule applies to the traffic from a pod getting routed back to another pod on same node by service proxy
	args = []string{"-I", kubeOutputChainName, "1", "-m", "comment", "--comment", comment, "-d", pod.ip, "-j", podFwChainName + "\n"}
	npc.filterTableRules.WriteString(strings.Join(args, " "))

	// ensure there is rule in filter table and forward chain to jump to pod specific firewall chain
	// this rule applies to the traffic getting switched (coming for same node pods)
	comment = "\"rule to jump traffic destined to POD name:" + pod.name + " namespace: " + pod.namespace +
		" to chain " + podFwChainName + "\""
	args = []string{"-I", kubeForwardChainName, "1", "-m", "physdev", "--physdev-is-bridged",
		"-m", "comment", "--comment", comment,
		"-d", pod.ip,
		"-j", podFwChainName, "\n"}
	npc.filterTableRules.WriteString(strings.Join(args, " "))
}

// setup iptable rules to intercept outbound traffic from pods and run it across the
// firewall chain corresponding to the pod so that egress network policies are enforced
func (npc *NetworkPolicyController) interceptPodOutboundTraffic(pod *podInfo, podFwChainName string) {
	egressFilterChains := []string{kubeInputChainName, kubeForwardChainName, kubeOutputChainName}
	for _, chain := range egressFilterChains {
		// ensure there is rule in filter table and FORWARD chain to jump to pod specific firewall chain
		// this rule applies to the traffic getting forwarded/routed (traffic from the pod destinted
		// to pod on a different node)
		comment := "\"rule to jump traffic from POD name:" + pod.name + " namespace: " + pod.namespace +
			" to chain " + podFwChainName + "\""
		args := []string{"-I", chain, "1", "-m", "comment", "--comment", comment, "-s", pod.ip, "-j", podFwChainName, "\n"}
		npc.filterTableRules.WriteString(strings.Join(args, " "))
	}

	// ensure there is rule in filter table and forward chain to jump to pod specific firewall chain
	// this rule applies to the traffic getting switched (coming for same node pods)
	comment := "\"rule to jump traffic from POD name:" + pod.name + " namespace: " + pod.namespace +
		" to chain " + podFwChainName + "\""
	args := []string{"-I", kubeForwardChainName, "1", "-m", "physdev", "--physdev-is-bridged",
		"-m", "comment", "--comment", comment,
		"-s", pod.ip,
		"-j", podFwChainName, "\n"}
	npc.filterTableRules.WriteString(strings.Join(args, " "))
}

func (npc *NetworkPolicyController) getLocalPods(nodeIP string) (*map[string]podInfo, error) {
	localPods := make(map[string]podInfo)
	for _, obj := range npc.podLister.List() {
		pod := obj.(*api.Pod)
		// ignore the pods running on the different node and pods that are not actionable
		if strings.Compare(pod.Status.HostIP, nodeIP) != 0 || !isNetPolActionable(pod) {
			continue
		}
		localPods[pod.Status.PodIP] = podInfo{ip: pod.Status.PodIP,
			name:      pod.ObjectMeta.Name,
			namespace: pod.ObjectMeta.Namespace,
			labels:    pod.ObjectMeta.Labels}
	}
	return &localPods, nil
}

func podFirewallChainName(namespace, podName string, version string) string {
	hash := sha256.Sum256([]byte(namespace + podName + version))
	encoded := base32.StdEncoding.EncodeToString(hash[:])
	return kubePodFirewallChainPrefix + encoded[:16]
}
