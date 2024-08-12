// This test verifies that two nodes, which can't connect using the local network, are
// able to still connect using the node-external-ip. In real life, node-external-ip
// would be a public IP. In the test, we create two networks, one sets the node
// internal-ip and the other sets the node-external-ip. Traffic is blocked on the former

package externalip

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 1, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var hardened = flag.Bool("hardened", false, "true or false")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// getLBServiceIPs returns the externalIP configured for flannel
func getExternalIPs(kubeConfigFile string) ([]string, error) {
	cmd := `kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.annotations.flannel\.alpha\.coreos\.com/public-ip-overwrite}'  --kubeconfig=` + kubeConfigFile
	res, err := e2e.RunCommand(cmd)
	if err != nil {
		return nil, err
	}
	return strings.Split(res, " "), nil
}

// getLBServiceIPs returns the LoadBalance service IPs
func getLBServiceIPs(kubeConfigFile string) ([]e2e.ObjIP, error) {
	cmd := `kubectl get svc -l k8s-app=nginx-app-loadbalancer -o=jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.loadBalancer.ingress[*].ip}{"\n"}{end}' --kubeconfig=` + kubeConfigFile
	return e2e.GetObjIPs(cmd)
}

// getClientIPs returns the IPs of the client pods
func getClientIPs(kubeConfigFile string) ([]e2e.ObjIP, error) {
	cmd := `kubectl get pods -l app=client -o=jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.podIPs[*].ip}{"\n"}{end}' --kubeconfig=` + kubeConfigFile
	return e2e.GetObjIPs(cmd)
}

func Test_E2EExternalIP(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "External-IP config Suite", suiteConfig, reporterConfig)

}

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify External-IP config", Ordered, func() {

	It("Starts up with no issues", func() {
		var err error
		if *local {
			serverNodeNames, agentNodeNames, err = e2e.CreateLocalCluster(*nodeOS, *serverCount, *agentCount)
		} else {
			serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
		}
		Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
		fmt.Println("CLUSTER CONFIG")
		fmt.Println("OS:", *nodeOS)
		fmt.Println("Server Nodes:", serverNodeNames)
		fmt.Println("Agent Nodes:", agentNodeNames)
		kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred())
	})

	It("Checks Node Status", func() {
		Eventually(func(g Gomega) {
			nodes, err := e2e.ParseNodes(kubeConfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, node := range nodes {
				g.Expect(node.Status).Should(Equal("Ready"))
			}
		}, "620s", "5s").Should(Succeed())
		_, err := e2e.ParseNodes(kubeConfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Checks Pod Status", func() {
		Eventually(func(g Gomega) {
			pods, err := e2e.ParsePods(kubeConfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods {
				if strings.Contains(pod.Name, "helm-install") {
					g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
				} else {
					g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
				}
			}
		}, "620s", "5s").Should(Succeed())
		_, err := e2e.ParsePods(kubeConfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Verifies that each node has vagrant IP", func() {
		nodeIPs, err := e2e.GetNodeIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, node := range nodeIPs {
			Expect(node.IPv4).Should(ContainSubstring("10.10."))
		}
	})
	It("Verifies that each pod has vagrant IP or clusterCIDR IP", func() {
		podIPs, err := e2e.GetPodIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, pod := range podIPs {
			Expect(pod.IPv4).Should(Or(ContainSubstring("10.10."), ContainSubstring("10.42.")), pod.Name)
		}
	})
	It("Verifies that flannel added the correct annotation for the external-ip", func() {
		nodeIPs, err := getExternalIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, annotation := range nodeIPs {
			Expect(annotation).Should(ContainSubstring("10.100.100."))
		}
	})
	It("Verifies internode connectivity over the tunnel", func() {
		_, err := e2e.DeployWorkload("pod_client.yaml", kubeConfigFile, *hardened)
		Expect(err).NotTo(HaveOccurred())

		// Wait for the pod_client to have an IP
		Eventually(func() string {
			ips, _ := getClientIPs(kubeConfigFile)
			return ips[0].IPv4
		}, "40s", "5s").Should(ContainSubstring("10.42"), "failed getClientIPs")

		clientIPs, err := getClientIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, ip := range clientIPs {
			cmd := "kubectl exec svc/client-curl --kubeconfig=" + kubeConfigFile + " -- curl -m7 " + ip.IPv4 + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "20s", "3s").Should(ContainSubstring("client-deployment"), "failed cmd: "+cmd)
		}
	})
	It("Verifies loadBalancer service's IP is the node-external-ip", func() {
		_, err := e2e.DeployWorkload("loadbalancer.yaml", kubeConfigFile, *hardened)
		Expect(err).NotTo(HaveOccurred())
		cmd := "kubectl --kubeconfig=" + kubeConfigFile + " get svc -l k8s-app=nginx-app-loadbalancer -o=jsonpath='{range .items[*]}{.metadata.name}{.status.loadBalancer.ingress[*].ip}{end}'"
		Eventually(func() (string, error) {
			return e2e.RunCommand(cmd)
		}, "20s", "3s").Should(ContainSubstring("10.100.100"), "failed cmd: "+cmd)
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if !failed {
		Expect(e2e.GetCoverageReport(append(serverNodeNames, agentNodeNames...))).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
