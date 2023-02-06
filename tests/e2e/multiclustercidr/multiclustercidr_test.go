package multiclustercidr

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

// Valid nodeOS: generic/ubuntu2004, opensuse/Leap-15.3.x86_64
var nodeOS = flag.String("nodeOS", "generic/ubuntu2004", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var hardened = flag.Bool("hardened", false, "true or false")
var ci = flag.Bool("ci", false, "running on CI")

func Test_E2EMultiClusterCIDR(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "MultiClusterCIDR Test Suite", suiteConfig, reporterConfig)
}

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify MultiClusterCIDR Configuration", Ordered, func() {

	It("Starts up IPv4 setup with no issues", func() {
		var err error
		os.Setenv("E2E_IP_FAMILY", "ipv4")
		defer os.Unsetenv("E2E_IP_FAMILY")
		serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
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
		}, "420s", "5s").Should(Succeed())
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
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParsePods(kubeConfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Verifies that each node has IPv4", func() {
		nodeIPs, err := e2e.GetNodeIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, node := range nodeIPs {
			Expect(node.IPv4).Should(ContainSubstring("10.10.10"))
		}
	})

	It("Verifies that each pod has IPv4", func() {
		podIPs, err := e2e.GetPodIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, pod := range podIPs {
			Expect(pod.IPv4).Should(Or(ContainSubstring("10.10.10"), ContainSubstring("10.42.")), pod.Name)
		}
	})

	It("Add new CIDR", func() {
		_, err := e2e.DeployWorkload("cluster-cidr.yaml", kubeConfigFile, *hardened)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() (string, error) {
			cmd := "kubectl get clustercidr new-cidr --kubeconfig=" + kubeConfigFile
			return e2e.RunCommand(cmd)
		}, "120s", "5s").Should(ContainSubstring("10.248.0.0"))

	})

	It("Restart agent-0", func() {
		agents := []string{"agent-0"}
		err := e2e.RestartClusterAgent(agents)
		Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
	})

	It("Checks Node Status", func() {
		Eventually(func(g Gomega) {
			nodes, err := e2e.ParseNodes(kubeConfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, node := range nodes {
				g.Expect(node.Status).Should(Equal("Ready"))
			}
		}, "420s", "5s").Should(Succeed())
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
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParsePods(kubeConfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Verifies that each pod of agent-0 has IPv4 from the new CIDR", func() {
		pods, err := e2e.ParsePods(kubeConfigFile, false)
		Expect(err).NotTo(HaveOccurred())
		for _, pod := range pods {
			if pod.Node == "agent-0" {
				Expect(pod.NodeIP).Should(Or(ContainSubstring("10.10.10"), ContainSubstring("10.248.")), pod.Name)
			}
		}
	})

	It("Destroy Cluster", func() {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	})

	It("Starts up IPv6 setup with no issues", func() {
		var err error
		os.Setenv("E2E_IP_FAMILY", "ipv6")
		defer os.Unsetenv("E2E_IP_FAMILY")
		serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
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
		}, "420s", "5s").Should(Succeed())
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
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParsePods(kubeConfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Verifies that each node has IPv6", func() {
		nodeIPs, err := e2e.GetNodeIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, node := range nodeIPs {
			Expect(node.IPv6).Should(ContainSubstring("fd11:decf:c0ff"))
		}
	})

	It("Verifies that each pod has IPv6", func() {
		podIPs, err := e2e.GetPodIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, pod := range podIPs {
			Expect(pod.IPv6).Should(Or(ContainSubstring("fd11:decf:c0ff"), ContainSubstring("2001:cafe:42")), pod.Name)
		}
	})

	It("Add new CIDR", func() {
		_, err := e2e.DeployWorkload("cluster-cidr-ipv6.yaml", kubeConfigFile, *hardened)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() (string, error) {
			cmd := "kubectl get clustercidr new-cidr --kubeconfig=" + kubeConfigFile
			return e2e.RunCommand(cmd)
		}, "120s", "5s").Should(ContainSubstring("2001:cafe:248"))

	})

	It("Delete and restart agent-0", func() {
		agents := []string{"agent-0"}
		err := e2e.RestartClusterAgent(agents)
		Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
	})

	It("Checks Node Status", func() {
		Eventually(func(g Gomega) {
			nodes, err := e2e.ParseNodes(kubeConfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, node := range nodes {
				g.Expect(node.Status).Should(Equal("Ready"))
			}
		}, "420s", "5s").Should(Succeed())
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
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParsePods(kubeConfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Verifies that each pod of agent-0 has IPv6 from the new CIDR", func() {
		pods, err := e2e.ParsePods(kubeConfigFile, false)
		Expect(err).NotTo(HaveOccurred())
		for _, pod := range pods {
			if pod.Node == "agent-0" {
				Expect(pod.NodeIP).Should(Or(ContainSubstring("fd11:decf:c0ff"), ContainSubstring("2001:cafe:248")), pod.Name)
			}
		}
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed && !*ci {
		fmt.Println("FAILED!")
	} else {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
