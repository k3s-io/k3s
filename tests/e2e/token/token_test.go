package snapshotrestore

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS:
// bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
// eurolinux-vagrant/rocky-8, eurolinux-vagrant/rocky-9,

var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 2, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.27.1+k3s2 (default: latest commit from master)

func Test_E2EToken(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "SnapshotRestore Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Use the token CLI to create and join agents", Ordered, func() {
	Context("Agent joins with permanent token:", func() {
		It("Starts up with no issues", func() {
			var err error
			if *local {
				tc, err = e2e.CreateLocalCluster(*nodeOS, *serverCount, *agentCount)
			} else {
				tc, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
			}
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			By("CLUSTER CONFIG")
			By("OS: " + *nodeOS)
			By(tc.Status())

		})

		It("Checks Node and Pod Status", func() {
			By("Fetching Nodes status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeConfigFile)

			By("Fetching Pods status")
			Eventually(func(g Gomega) {
				pods, err := e2e.ParsePods(tc.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
					} else {
						g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
					}
				}
			}, "420s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeConfigFile)
		})

		var permToken string
		It("Creates a permanent agent token", func() {
			permToken = "perage.s0xt4u0hl5guoyi6"
			_, err := tc.Servers[0].RunCmdOnNode("k3s token create --ttl=0 " + permToken)
			Expect(err).NotTo(HaveOccurred())

			res, err := tc.Servers[0].RunCmdOnNode("k3s token list")
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(MatchRegexp(`perage\s+<forever>\s+<never>`))
		})
		It("Joins an agent with the permanent token", func() {
			cmd := fmt.Sprintf("echo 'token: %s' | sudo tee -a /etc/rancher/k3s/config.yaml > /dev/null", permToken)
			_, err := tc.Agents[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred())
			_, err = tc.Agents[0].RunCmdOnNode("systemctl start k3s-agent")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).Should(Equal(len(tc.Servers) + 1))
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "60s", "5s").Should(Succeed())
		})
	})
	Context("Agent joins with temporary token:", func() {
		It("Creates a 20s agent token", func() {
			_, err := tc.Servers[0].RunCmdOnNode("k3s token create --ttl=20s 20sect.jxnpve6vg8dqm895")
			Expect(err).NotTo(HaveOccurred())
			res, err := tc.Servers[0].RunCmdOnNode("k3s token list")
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(MatchRegexp(`20sect\s+[0-9]{2}s`))
		})
		It("Cleans up 20s token automatically", func() {
			Eventually(func() (string, error) {
				return tc.Servers[0].RunCmdOnNode("k3s token list")
			}, "25s", "5s").ShouldNot(ContainSubstring("20sect"))
		})
		var tempToken string
		It("Creates a 10m agent token", func() {
			tempToken = "10mint.ida18trbbk43szwk"
			_, err := tc.Servers[0].RunCmdOnNode("k3s token create --ttl=10m " + tempToken)
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(2 * time.Second)
			res, err := tc.Servers[0].RunCmdOnNode("k3s token list")
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(MatchRegexp(`10mint\s+[0-9]m`))
		})
		It("Joins an agent with the 10m token", func() {
			cmd := fmt.Sprintf("echo 'token: %s' | sudo tee -a /etc/rancher/k3s/config.yaml > /dev/null", tempToken)
			_, err := tc.Agents[1].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred())
			_, err = tc.Agents[1].RunCmdOnNode("systemctl start k3s-agent")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).Should(Equal(len(tc.Servers) + 2))
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "60s", "5s").Should(Succeed())
		})
	})
	Context("Rotate server bootstrap token", func() {
		serverToken := "1234"
		It("Creates a new server token", func() {
			Expect(tc.Servers[0].RunCmdOnNode("k3s token rotate -t vagrant --new-token=" + serverToken)).
				To(ContainSubstring("Token rotated, restart k3s nodes with new token"))
		})
		It("Restarts servers with the new token", func() {
			cmd := fmt.Sprintf("sed -i 's/token:.*/token: %s/' /etc/rancher/k3s/config.yaml", serverToken)
			for _, node := range tc.Servers {
				_, err := node.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred())
			}
			for _, node := range tc.Servers {
				_, err := node.RunCmdOnNode("systemctl restart k3s")
				Expect(err).NotTo(HaveOccurred())
			}
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).Should(Equal(len(tc.Servers) + 2))
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "60s", "5s").Should(Succeed())
		})
		It("Rejoins an agent with the new server token", func() {
			cmd := fmt.Sprintf("sed -i 's/token:.*/token: %s/' /etc/rancher/k3s/config.yaml", serverToken)
			_, err := tc.Agents[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred())
			_, err = tc.Agents[0].RunCmdOnNode("systemctl restart k3s-agent")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).Should(Equal(len(tc.Servers) + 2))
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "60s", "5s").Should(Succeed())
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		AddReportEntry("journald-logs", e2e.TailJournalLogs(1000, append(tc.Servers, tc.Agents...)))
	} else {
		Expect(e2e.GetCoverageReport(append(tc.Servers, tc.Agents...))).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeConfigFile)).To(Succeed())
	}
})
