package rotateca

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
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 or nil for latest commit from master

func Test_E2ECustomCARotation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Custom Certificate Rotation Test Suite", suiteConfig, reporterConfig)
}

var (
	kubeConfigFile string
	serverNodes    []e2e.VagrantNode
	agentNodes     []e2e.VagrantNode
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Custom CA Rotation", Ordered, func() {
	Context("Custom CA is rotated:", func() {
		It("Starts up with no issues", func() {
			var err error
			if *local {
				serverNodes, agentNodes, err = e2e.CreateLocalCluster(*nodeOS, *serverCount, *agentCount)
			} else {
				serverNodes, agentNodes, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
			}
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			fmt.Println("CLUSTER CONFIG")
			fmt.Println("OS:", *nodeOS)
			fmt.Println("Server Nodes:", serverNodes)
			fmt.Println("Agent Nodes:", agentNodes)
			kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodes[0].String())
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status", func() {
			By("Fetching Nodes status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
			e2e.DumpPods(kubeConfigFile)

			By("Fetching Pods status")
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
			e2e.DumpPods(kubeConfigFile)
		})

		It("Generates New CA Certificates", func() {
			cmds := []string{
				"mkdir -p /opt/rancher/k3s/server",
				"cp -r /var/lib/rancher/k3s/server/tls /opt/rancher/k3s/server",
				"DATA_DIR=/opt/rancher/k3s /tmp/generate-custom-ca-certs.sh",
			}
			for _, cmd := range cmds {
				_, err := serverNodes[0].RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("Rotates CA Certificates", func() {
			cmd := "k3s certificate rotate-ca --path=/opt/rancher/k3s/server"
			_, err := serverNodes[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Restarts K3s servers", func() {
			Expect(e2e.RestartCluster(serverNodes)).To(Succeed())
		})

		It("Restarts K3s agents", func() {
			Expect(e2e.RestartCluster(agentNodes)).To(Succeed())
		})

		It("Checks node and pod status", func() {
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())

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
			e2e.DumpPods(kubeConfigFile)
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		AddReportEntry("journald-logs", e2e.TailJournalLogs(1000, append(serverNodes, agentNodes...)))
	} else {
		Expect(e2e.GetCoverageReport(append(serverNodes, agentNodes...))).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
