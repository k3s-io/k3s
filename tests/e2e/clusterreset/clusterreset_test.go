package clusterreset

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

// Valid nodeOS:
// generic/ubuntu2004, generic/centos7, generic/rocky8,
// opensuse/Leap-15.3.x86_64, dweomer/microos.amd64
var nodeOS = flag.String("nodeOS", "generic/ubuntu2004", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 2, "number of agent nodes")
var hardened = flag.Bool("hardened", false, "true or false")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 (default: latest commit from master)

func Test_E2EClusterReset(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	RunSpecs(t, "Create ClusterReset Test Suite")
}

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

var _ = Describe("Verify Create", Ordered, func() {
	Context("Cluster :", func() {
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

		It("Checks Node and Pod Status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(kubeConfigFile, true)

			fmt.Printf("\nFetching Pods status\n")
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
			_, _ = e2e.ParsePods(kubeConfigFile, true)
		})

		It("Verifies ClusterReset Functionality", func() {
			Eventually(func(g Gomega) {
				for _, nodeName := range serverNodeNames {
					if nodeName != "server-0" {
						cmd := "sudo systemctl stop k3s"
						_, err := e2e.RunCmdOnNode(cmd, nodeName)
						Expect(err).NotTo(HaveOccurred())
					}
				}

				cmd := "sudo systemctl stop k3s"
				_, err := e2e.RunCmdOnNode(cmd, "server-0")
				Expect(err).NotTo(HaveOccurred())

				cmd = "sudo k3s server --cluster-reset"
				res, err := e2e.RunCmdOnNode(cmd, "server-0")
				Expect(err).NotTo(HaveOccurred())
				Expect(res).Should(ContainSubstring("Managed etcd cluster membership has been reset, restart without --cluster-reset flag now"))

				cmd = "sudo systemctl start k3s"
				_, err = e2e.RunCmdOnNode(cmd, "server-0")
				Expect(err).NotTo(HaveOccurred())

				fmt.Printf("\nFetching node status\n")
				Eventually(func(g Gomega) {
					nodes, err := e2e.ParseNodes(kubeConfigFile, false)
					g.Expect(err).NotTo(HaveOccurred())
					for _, node := range nodes {
						if strings.Contains(node.Name, "server-0") || strings.Contains(node.Name, "agent-") {
							g.Expect(node.Status).Should(Equal("Ready"))
						} else {
							g.Expect(node.Status).Should(Equal("NotReady"))
						}
					}
				}, "480s", "5s").Should(Succeed())
				_, _ = e2e.ParseNodes(kubeConfigFile, true)

				fmt.Printf("\nFetching Pods status\n")
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
				_, _ = e2e.ParsePods(kubeConfigFile, true)
				for _, nodeName := range serverNodeNames {
					if nodeName != "server-0" {
						cmd := "sudo rm -rf /var/lib/rancher/k3s/server/db"
						_, err := e2e.RunCmdOnNode(cmd, nodeName)
						Expect(err).NotTo(HaveOccurred())
						cmd = "sudo systemctl restart k3s"
						_, err = e2e.RunCmdOnNode(cmd, nodeName)
						Expect(err).NotTo(HaveOccurred())
					}
				}
				Eventually(func(g Gomega) {
					nodes, err := e2e.ParseNodes(kubeConfigFile, false)
					g.Expect(err).NotTo(HaveOccurred())
					for _, node := range nodes {
						g.Expect(node.Status).Should(Equal("Ready"))
					}
				}, "420s", "5s").Should(Succeed())
				_, _ = e2e.ParseNodes(kubeConfigFile, true)

				fmt.Printf("\nFetching Pods status\n")
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
				_, _ = e2e.ParsePods(kubeConfigFile, true)
			}, "240s", "5s").Should(Succeed())
		})
	})
})

var failed = false
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
