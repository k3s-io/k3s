// Apache License v2.0 (copyright Cloud Native Labs & Rancher Labs)
// - modified from https://github.com/cloudnativelabs/kube-router/blob/73b1b03b32c5755b240f6c077bb097abe3888314/pkg/controllers/network_policy_controller_test.go

// +build !windows

package netpol

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rancher/k3s/pkg/daemons/config"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// newFakeInformersFromClient creates the different informers used in the uneventful network policy controller
func newFakeInformersFromClient(kubeClient clientset.Interface) (informers.SharedInformerFactory, cache.SharedIndexInformer, cache.SharedIndexInformer, cache.SharedIndexInformer) {
	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	podInformer := informerFactory.Core().V1().Pods().Informer()
	npInformer := informerFactory.Networking().V1().NetworkPolicies().Informer()
	nsInformer := informerFactory.Core().V1().Namespaces().Informer()
	return informerFactory, podInformer, nsInformer, npInformer
}

type tNamespaceMeta struct {
	name   string
	labels labels.Set
}

// Add resources to Informer Store object to simulate updating the Informer
func tAddToInformerStore(t *testing.T, informer cache.SharedIndexInformer, obj interface{}) {
	err := informer.GetStore().Add(obj)
	if err != nil {
		t.Fatalf("error injecting object to Informer Store: %v", err)
	}
}

type tNetpol struct {
	name        string
	namespace   string
	podSelector metav1.LabelSelector
	ingress     []netv1.NetworkPolicyIngressRule
	egress      []netv1.NetworkPolicyEgressRule
}

// createFakeNetpol is a helper to create the network policy from the tNetpol struct
func (ns *tNetpol) createFakeNetpol(t *testing.T, informer cache.SharedIndexInformer) {
	polTypes := make([]netv1.PolicyType, 0)
	if len(ns.ingress) != 0 {
		polTypes = append(polTypes, netv1.PolicyTypeIngress)
	}
	if len(ns.egress) != 0 {
		polTypes = append(polTypes, netv1.PolicyTypeEgress)
	}
	tAddToInformerStore(t, informer,
		&netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: ns.name, Namespace: ns.namespace},
			Spec: netv1.NetworkPolicySpec{
				PodSelector: ns.podSelector,
				PolicyTypes: polTypes,
				Ingress:     ns.ingress,
				Egress:      ns.egress,
			}})
}

func (ns *tNetpol) findNetpolMatch(netpols *[]networkPolicyInfo) *networkPolicyInfo {
	for _, netpol := range *netpols {
		if netpol.namespace == ns.namespace && netpol.name == ns.name {
			return &netpol
		}
	}
	return nil
}

// tPodNamespaceMap is a helper to create sets of namespace,pod names
type tPodNamespaceMap map[string]map[string]bool

func (t tPodNamespaceMap) addPod(pod podInfo) {
	if _, ok := t[pod.namespace]; !ok {
		t[pod.namespace] = make(map[string]bool)
	}
	t[pod.namespace][pod.name] = true
}
func (t tPodNamespaceMap) delPod(pod podInfo) {
	delete(t[pod.namespace], pod.name)
	if len(t[pod.namespace]) == 0 {
		delete(t, pod.namespace)
	}
}
func (t tPodNamespaceMap) addNSPodInfo(ns, podname string) {
	if _, ok := t[ns]; !ok {
		t[ns] = make(map[string]bool)
	}
	t[ns][podname] = true
}
func (t tPodNamespaceMap) copy() tPodNamespaceMap {
	newMap := make(tPodNamespaceMap)
	for ns, pods := range t {
		for p := range pods {
			newMap.addNSPodInfo(ns, p)
		}
	}
	return newMap
}
func (t tPodNamespaceMap) toStrSlice() (r []string) {
	for ns, pods := range t {
		for pod := range pods {
			r = append(r, ns+":"+pod)
		}
	}
	return
}

// tNewPodNamespaceMapFromTC creates a new tPodNamespaceMap from the info of the testcase
func tNewPodNamespaceMapFromTC(target map[string]string) tPodNamespaceMap {
	newMap := make(tPodNamespaceMap)
	for ns, pods := range target {
		for _, pod := range strings.Split(pods, ",") {
			newMap.addNSPodInfo(ns, pod)
		}
	}
	return newMap
}

// tCreateFakePods creates the Pods and Namespaces that will be affected by the network policies
//	returns a map like map[Namespace]map[PodName]bool
func tCreateFakePods(t *testing.T, podInformer cache.SharedIndexInformer, nsInformer cache.SharedIndexInformer) tPodNamespaceMap {
	podNamespaceMap := make(tPodNamespaceMap)
	pods := []podInfo{
		{name: "Aa", labels: labels.Set{"app": "a"}, namespace: "nsA", ip: "1.1"},
		{name: "Aaa", labels: labels.Set{"app": "a", "component": "a"}, namespace: "nsA", ip: "1.2"},
		{name: "Aab", labels: labels.Set{"app": "a", "component": "b"}, namespace: "nsA", ip: "1.3"},
		{name: "Aac", labels: labels.Set{"app": "a", "component": "c"}, namespace: "nsA", ip: "1.4"},
		{name: "Ba", labels: labels.Set{"app": "a"}, namespace: "nsB", ip: "2.1"},
		{name: "Baa", labels: labels.Set{"app": "a", "component": "a"}, namespace: "nsB", ip: "2.2"},
		{name: "Bab", labels: labels.Set{"app": "a", "component": "b"}, namespace: "nsB", ip: "2.3"},
		{name: "Ca", labels: labels.Set{"app": "a"}, namespace: "nsC", ip: "3.1"},
	}
	namespaces := []tNamespaceMeta{
		{name: "nsA", labels: labels.Set{"name": "a", "team": "a"}},
		{name: "nsB", labels: labels.Set{"name": "b", "team": "a"}},
		{name: "nsC", labels: labels.Set{"name": "c"}},
		{name: "nsD", labels: labels.Set{"name": "d"}},
	}
	ipsUsed := make(map[string]bool)
	for _, pod := range pods {
		podNamespaceMap.addPod(pod)
		ipaddr := "1.1." + pod.ip
		if ipsUsed[ipaddr] {
			t.Fatalf("there is another pod with the same Ip address %s as this pod %s namespace %s",
				ipaddr, pod.name, pod.name)
		}
		ipsUsed[ipaddr] = true
		tAddToInformerStore(t, podInformer,
			&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pod.name, Labels: pod.labels, Namespace: pod.namespace},
				Status: v1.PodStatus{PodIP: ipaddr}})
	}
	for _, ns := range namespaces {
		tAddToInformerStore(t, nsInformer, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns.name, Labels: ns.labels}})
	}
	return podNamespaceMap
}

// newFakeNode is a helper function for creating Nodes for testing.
func newFakeNode(name string, addr string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceCPU):    resource.MustParse("1"),
				v1.ResourceName(v1.ResourceMemory): resource.MustParse("1G"),
			},
			Addresses: []v1.NodeAddress{{Type: v1.NodeExternalIP, Address: addr}},
		},
	}
}

// newUneventfulNetworkPolicyController returns new NetworkPolicyController object without any event handler
func newUneventfulNetworkPolicyController(podInformer cache.SharedIndexInformer,
	npInformer cache.SharedIndexInformer, nsInformer cache.SharedIndexInformer) (*NetworkPolicyController, error) {

	npc := NetworkPolicyController{}
	npc.syncPeriod = time.Hour

	npc.nodeHostName = "node"
	npc.nodeIP = net.IPv4(10, 10, 10, 10)
	npc.podLister = podInformer.GetIndexer()
	npc.nsLister = nsInformer.GetIndexer()
	npc.npLister = npInformer.GetIndexer()

	return &npc, nil
}

// tNetpolTestCase helper struct to define the inputs to the test case (netpols) and
// 				  the expected selected targets (targetPods, inSourcePods for ingress targets, and outDestPods
//				  for egress targets) as maps with key being the namespace and a csv of pod names
type tNetpolTestCase struct {
	name         string
	netpol       tNetpol
	targetPods   tPodNamespaceMap
	inSourcePods tPodNamespaceMap
	outDestPods  tPodNamespaceMap
	expectedRule string
}

// tGetNotTargetedPods finds set of pods that should not be targeted by netpol selectors
func tGetNotTargetedPods(podsGot []podInfo, wanted tPodNamespaceMap) []string {
	unwanted := make(tPodNamespaceMap)
	for _, pod := range podsGot {
		if !wanted[pod.namespace][pod.name] {
			unwanted.addPod(pod)
		}
	}
	return unwanted.toStrSlice()
}

// tGetTargetPodsMissing returns the set of pods that should have been targeted but were missing by netpol selectors
func tGetTargetPodsMissing(podsGot []podInfo, wanted tPodNamespaceMap) []string {
	missing := wanted.copy()
	for _, pod := range podsGot {
		if wanted[pod.namespace][pod.name] {
			missing.delPod(pod)
		}
	}
	return missing.toStrSlice()
}

func tListOfPodsFromTargets(target map[string]podInfo) (r []podInfo) {
	for _, pod := range target {
		r = append(r, pod)
	}
	return
}

func testForMissingOrUnwanted(t *testing.T, targetMsg string, got []podInfo, wanted tPodNamespaceMap) {
	if missing := tGetTargetPodsMissing(got, wanted); len(missing) != 0 {
		t.Errorf("Some Pods were not selected %s: %s", targetMsg, strings.Join(missing, ", "))
	}
	if missing := tGetNotTargetedPods(got, wanted); len(missing) != 0 {
		t.Errorf("Some Pods NOT expected were selected on %s: %s", targetMsg, strings.Join(missing, ", "))
	}
}

func newMinimalNodeConfig(serviceIPCIDR string, nodePortRange string, hostNameOverride string, externalIPs []string) *config.Node {
	nodeConfig := &config.Node{AgentConfig: config.Agent{}}

	if serviceIPCIDR != "" {
		_, cidr, err := net.ParseCIDR(serviceIPCIDR)
		if err != nil {
			panic("failed to get parse --service-cluster-ip-range parameter: " + err.Error())
		}
		nodeConfig.AgentConfig.ServiceCIDR = cidr
	} else {
		nodeConfig.AgentConfig.ServiceCIDR = &net.IPNet{}
	}
	if nodePortRange != "" {
		portRange, err := utilnet.ParsePortRange(nodePortRange)
		if err != nil {
			panic("failed to get parse --service-node-port-range:" + err.Error())
		}
		nodeConfig.AgentConfig.ServiceNodePortRange = *portRange
	}
	if hostNameOverride != "" {
		nodeConfig.AgentConfig.NodeName = hostNameOverride
	}
	if externalIPs != nil {
		// TODO: We don't currently have a way to set these through the K3s CLI; if we ever do then test that here.
		for _, cidr := range externalIPs {
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				panic("failed to get parse --service-external-ip-range parameter: " + err.Error())
			}
		}
	}
	return nodeConfig
}

type tNetPolConfigTestCase struct {
	name        string
	config      *config.Node
	expectError bool
	errorText   string
}

func TestNewNetworkPolicySelectors(t *testing.T) {
	testCases := []tNetpolTestCase{
		{
			name:       "Non-Existent Namespace",
			netpol:     tNetpol{name: "nsXX", podSelector: metav1.LabelSelector{}, namespace: "nsXX"},
			targetPods: nil,
		},
		{
			name:       "Empty Namespace",
			netpol:     tNetpol{name: "nsD", podSelector: metav1.LabelSelector{}, namespace: "nsD"},
			targetPods: nil,
		},
		{
			name:       "All pods in nsA",
			netpol:     tNetpol{name: "nsA", podSelector: metav1.LabelSelector{}, namespace: "nsA"},
			targetPods: tNewPodNamespaceMapFromTC(map[string]string{"nsA": "Aa,Aaa,Aab,Aac"}),
		},
		{
			name:       "All pods in nsB",
			netpol:     tNetpol{name: "nsB", podSelector: metav1.LabelSelector{}, namespace: "nsB"},
			targetPods: tNewPodNamespaceMapFromTC(map[string]string{"nsB": "Ba,Baa,Bab"}),
		},
		{
			name:       "All pods in nsC",
			netpol:     tNetpol{name: "nsC", podSelector: metav1.LabelSelector{}, namespace: "nsC"},
			targetPods: tNewPodNamespaceMapFromTC(map[string]string{"nsC": "Ca"}),
		},
		{
			name: "All pods app=a in nsA using matchExpressions",
			netpol: tNetpol{
				name:      "nsA-app-a-matchExpression",
				namespace: "nsA",
				podSelector: metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      "app",
						Operator: "In",
						Values:   []string{"a"},
					}}}},
			targetPods: tNewPodNamespaceMapFromTC(map[string]string{"nsA": "Aa,Aaa,Aab,Aac"}),
		},
		{
			name: "All pods app=a in nsA using matchLabels",
			netpol: tNetpol{name: "nsA-app-a-matchLabels", namespace: "nsA",
				podSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "a"}}},
			targetPods: tNewPodNamespaceMapFromTC(map[string]string{"nsA": "Aa,Aaa,Aab,Aac"}),
		},
		{
			name: "All pods app=a in nsA using matchLabels ingress allow from any pod in nsB",
			netpol: tNetpol{name: "nsA-app-a-matchLabels-2", namespace: "nsA",
				podSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "a"}},
				ingress:     []netv1.NetworkPolicyIngressRule{{From: []netv1.NetworkPolicyPeer{{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": "b"}}}}}},
			},
			targetPods:   tNewPodNamespaceMapFromTC(map[string]string{"nsA": "Aa,Aaa,Aab,Aac"}),
			inSourcePods: tNewPodNamespaceMapFromTC(map[string]string{"nsB": "Ba,Baa,Bab"}),
		},
		{
			name: "All pods app=a in nsA using matchLabels ingress allow from pod in nsB with component = b",
			netpol: tNetpol{name: "nsA-app-a-matchExpression-2", namespace: "nsA",
				podSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "a"}},
				ingress: []netv1.NetworkPolicyIngressRule{{From: []netv1.NetworkPolicyPeer{
					{
						NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": "b"}},
						PodSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{{
								Key:      "component",
								Operator: "In",
								Values:   []string{"b"},
							}}},
					},
				}}}},
			targetPods:   tNewPodNamespaceMapFromTC(map[string]string{"nsA": "Aa,Aaa,Aab,Aac"}),
			inSourcePods: tNewPodNamespaceMapFromTC(map[string]string{"nsB": "Bab"}),
		},
		{
			name: "All pods app=a,component=b or c in nsA",
			netpol: tNetpol{name: "nsA-app-a-matchExpression-3", namespace: "nsA",
				podSelector: metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "app",
							Operator: "In",
							Values:   []string{"a"},
						},
						{
							Key:      "component",
							Operator: "In",
							Values:   []string{"b", "c"},
						}}},
			},
			targetPods: tNewPodNamespaceMapFromTC(map[string]string{"nsA": "Aab,Aac"}),
		},
	}

	client := fake.NewSimpleClientset(&v1.NodeList{Items: []v1.Node{*newFakeNode("node", "10.10.10.10")}})
	informerFactory, podInformer, nsInformer, netpolInformer := newFakeInformersFromClient(client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	informerFactory.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced)
	krNetPol, _ := newUneventfulNetworkPolicyController(podInformer, netpolInformer, nsInformer)
	tCreateFakePods(t, podInformer, nsInformer)
	for _, test := range testCases {
		test.netpol.createFakeNetpol(t, netpolInformer)
	}
	netpols, err := krNetPol.buildNetworkPoliciesInfo()
	if err != nil {
		t.Errorf("Problems building policies")
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			np := test.netpol.findNetpolMatch(&netpols)
			testForMissingOrUnwanted(t, "targetPods", tListOfPodsFromTargets(np.targetPods), test.targetPods)
			for _, ingress := range np.ingressRules {
				testForMissingOrUnwanted(t, "ingress srcPods", ingress.srcPods, test.inSourcePods)
			}
			for _, egress := range np.egressRules {
				testForMissingOrUnwanted(t, "egress dstPods", egress.dstPods, test.outDestPods)
			}
		})
	}
}

func TestNetworkPolicyBuilder(t *testing.T) {
	port, port1 := intstr.FromInt(30000), intstr.FromInt(34000)
	ingressPort := intstr.FromInt(37000)
	endPort, endPort1 := int32(31000), int32(35000)
	testCases := []tNetpolTestCase{
		{
			name: "Simple Egress Destination Port",
			netpol: tNetpol{name: "simple-egress", namespace: "nsA",
				podSelector: metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "app",
							Operator: "In",
							Values:   []string{"a"},
						},
					},
				},
				egress: []netv1.NetworkPolicyEgressRule{
					{
						Ports: []netv1.NetworkPolicyPort{
							{
								Port: &port,
							},
						},
					},
				},
			},
			expectedRule: "-A KUBE-NWPLCY-QHFGOTFJZFXUJVTH -m comment --comment \"rule to ACCEPT traffic from source pods to all destinations selected by policy name: simple-egress namespace nsA\" --dport 30000 -j MARK --set-xmark 0x10000/0x10000 \n" +
				"-A KUBE-NWPLCY-QHFGOTFJZFXUJVTH -m comment --comment \"rule to ACCEPT traffic from source pods to all destinations selected by policy name: simple-egress namespace nsA\" --dport 30000 -m mark --mark 0x10000/0x10000 -j RETURN \n",
		},
		{
			name: "Simple Ingress/Egress Destination Port",
			netpol: tNetpol{name: "simple-ingress-egress", namespace: "nsA",
				podSelector: metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "app",
							Operator: "In",
							Values:   []string{"a"},
						},
					},
				},
				egress: []netv1.NetworkPolicyEgressRule{
					{
						Ports: []netv1.NetworkPolicyPort{
							{
								Port: &port,
							},
						},
					},
				},
				ingress: []netv1.NetworkPolicyIngressRule{
					{
						Ports: []netv1.NetworkPolicyPort{
							{
								Port: &ingressPort,
							},
						},
					},
				},
			},
			expectedRule: "-A KUBE-NWPLCY-KO52PWL34ABMMBI7 -m comment --comment \"rule to ACCEPT traffic from source pods to all destinations selected by policy name: simple-ingress-egress namespace nsA\" --dport 30000 -j MARK --set-xmark 0x10000/0x10000 \n" +
				"-A KUBE-NWPLCY-KO52PWL34ABMMBI7 -m comment --comment \"rule to ACCEPT traffic from source pods to all destinations selected by policy name: simple-ingress-egress namespace nsA\" --dport 30000 -m mark --mark 0x10000/0x10000 -j RETURN \n" +
				"-A KUBE-NWPLCY-KO52PWL34ABMMBI7 -m comment --comment \"rule to ACCEPT traffic from all sources to dest pods selected by policy name: simple-ingress-egress namespace nsA\" --dport 37000 -j MARK --set-xmark 0x10000/0x10000 \n" +
				"-A KUBE-NWPLCY-KO52PWL34ABMMBI7 -m comment --comment \"rule to ACCEPT traffic from all sources to dest pods selected by policy name: simple-ingress-egress namespace nsA\" --dport 37000 -m mark --mark 0x10000/0x10000 -j RETURN \n",
		},
		{
			name: "Simple Egress Destination Port Range",
			netpol: tNetpol{name: "simple-egress-pr", namespace: "nsA",
				podSelector: metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "app",
							Operator: "In",
							Values:   []string{"a"},
						},
					},
				},
				egress: []netv1.NetworkPolicyEgressRule{
					{
						Ports: []netv1.NetworkPolicyPort{
							{
								Port:    &port,
								EndPort: &endPort,
							},
							{
								Port:    &port1,
								EndPort: &endPort1,
							},
						},
					},
				},
			},
			expectedRule: "-A KUBE-NWPLCY-SQYQ7PVNG6A6Q3DU -m comment --comment \"rule to ACCEPT traffic from source pods to all destinations selected by policy name: simple-egress-pr namespace nsA\" --dport 30000:31000 -j MARK --set-xmark 0x10000/0x10000 \n" +
				"-A KUBE-NWPLCY-SQYQ7PVNG6A6Q3DU -m comment --comment \"rule to ACCEPT traffic from source pods to all destinations selected by policy name: simple-egress-pr namespace nsA\" --dport 30000:31000 -m mark --mark 0x10000/0x10000 -j RETURN \n" +
				"-A KUBE-NWPLCY-SQYQ7PVNG6A6Q3DU -m comment --comment \"rule to ACCEPT traffic from source pods to all destinations selected by policy name: simple-egress-pr namespace nsA\" --dport 34000:35000 -j MARK --set-xmark 0x10000/0x10000 \n" +
				"-A KUBE-NWPLCY-SQYQ7PVNG6A6Q3DU -m comment --comment \"rule to ACCEPT traffic from source pods to all destinations selected by policy name: simple-egress-pr namespace nsA\" --dport 34000:35000 -m mark --mark 0x10000/0x10000 -j RETURN \n",
		},
		{
			name: "Port > EndPort (invalid condition, should drop endport)",
			netpol: tNetpol{name: "invalid-endport", namespace: "nsA",
				podSelector: metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "app",
							Operator: "In",
							Values:   []string{"a"},
						},
					},
				},
				egress: []netv1.NetworkPolicyEgressRule{
					{
						Ports: []netv1.NetworkPolicyPort{
							{
								Port:    &port1,
								EndPort: &endPort,
							},
						},
					},
				},
			},
			expectedRule: "-A KUBE-NWPLCY-2A4DPWPR5REBS66I -m comment --comment \"rule to ACCEPT traffic from source pods to all destinations selected by policy name: invalid-endport namespace nsA\" --dport 34000 -j MARK --set-xmark 0x10000/0x10000 \n" +
				"-A KUBE-NWPLCY-2A4DPWPR5REBS66I -m comment --comment \"rule to ACCEPT traffic from source pods to all destinations selected by policy name: invalid-endport namespace nsA\" --dport 34000 -m mark --mark 0x10000/0x10000 -j RETURN \n",
		},
	}

	client := fake.NewSimpleClientset(&v1.NodeList{Items: []v1.Node{*newFakeNode("node", "10.10.10.10")}})
	informerFactory, podInformer, nsInformer, netpolInformer := newFakeInformersFromClient(client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	informerFactory.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced)
	krNetPol, _ := newUneventfulNetworkPolicyController(podInformer, netpolInformer, nsInformer)
	tCreateFakePods(t, podInformer, nsInformer)
	for _, test := range testCases {
		test.netpol.createFakeNetpol(t, netpolInformer)
		netpols, err := krNetPol.buildNetworkPoliciesInfo()
		if err != nil {
			t.Errorf("Problems building policies: %s", err)
		}
		for _, np := range netpols {
			fmt.Printf(np.policyType)
			if np.policyType == "egress" || np.policyType == "both" {
				err = krNetPol.processEgressRules(np, "", nil, "1")
				if err != nil {
					t.Errorf("Error syncing the rules: %s", err)
				}
			}
			if np.policyType == "ingress" || np.policyType == "both" {
				err = krNetPol.processIngressRules(np, "", nil, "1")
				if err != nil {
					t.Errorf("Error syncing the rules: %s", err)
				}
			}
		}

		if !bytes.Equal([]byte(test.expectedRule), krNetPol.filterTableRules.Bytes()) {
			t.Errorf("Invalid rule %s created:\nExpected:\n%s \nGot:\n%s", test.name, test.expectedRule, krNetPol.filterTableRules.String())
		}
		key := fmt.Sprintf("%s/%s", test.netpol.namespace, test.netpol.name)
		obj, exists, err := krNetPol.npLister.GetByKey(key)
		if err != nil {
			t.Errorf("Failed to get Netpol from store: %s", err)
		}
		if exists {
			err = krNetPol.npLister.Delete(obj)
			if err != nil {
				t.Errorf("Failed to remove Netpol from store: %s", err)
			}
		}
		krNetPol.filterTableRules.Reset()

	}

}

func TestNetworkPolicyController(t *testing.T) {
	testCases := []tNetPolConfigTestCase{
		{
			"Default options are successful",
			newMinimalNodeConfig("", "", "node", nil),
			false,
			"",
		},
		{
			"Missing nodename fails appropriately",
			newMinimalNodeConfig("", "", "", nil),
			true,
			"failed to identify the node by NODE_NAME, hostname or --hostname-override",
		},
		{
			"Test good cluster CIDR (using single IP with a /32)",
			newMinimalNodeConfig("10.10.10.10/32", "", "node", nil),
			false,
			"",
		},
		{
			"Test good cluster CIDR (using normal range with /24)",
			newMinimalNodeConfig("10.10.10.0/24", "", "node", nil),
			false,
			"",
		},
		{
			"Test good node port specification (using hyphen separator)",
			newMinimalNodeConfig("", "8080-8090", "node", nil),
			false,
			"",
		},
		{
			"Test good external IP CIDR (using single IP with a /32)",
			newMinimalNodeConfig("", "", "node", []string{"199.10.10.10/32"}),
			false,
			"",
		},
		{
			"Test good external IP CIDR (using normal range with /24)",
			newMinimalNodeConfig("", "", "node", []string{"199.10.10.10/24"}),
			false,
			"",
		},
	}
	client := fake.NewSimpleClientset(&v1.NodeList{Items: []v1.Node{*newFakeNode("node", "10.10.10.10")}})
	_, podInformer, nsInformer, netpolInformer := newFakeInformersFromClient(client)
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewNetworkPolicyController(client, test.config, podInformer, netpolInformer, nsInformer, &sync.Mutex{})
			if err == nil && test.expectError {
				t.Error("This config should have failed, but it was successful instead")
			} else if err != nil {
				// Unfortunately without doing a ton of extra refactoring work, we can't remove this reference easily
				// from the controllers start up. Luckily it's one of the last items to be processed in the controller
				// so for now we'll consider that if we hit this error that we essentially didn't hit an error at all
				// TODO: refactor NPC to use an injectable interface for ipset operations
				if !test.expectError && err.Error() != "Ipset utility not found" {
					t.Errorf("This config should have been successful, but it failed instead. Error: %s", err)
				} else if test.expectError && err.Error() != test.errorText {
					t.Errorf("Expected error: '%s' but instead got: '%s'", test.errorText, err)
				}
			}
		})
	}
}

// Ref:
// https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/podgc/gc_controller_test.go
// https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/testutil/test_utils.go
