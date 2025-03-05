package snapshotrestore

import (
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var ci = flag.Bool("ci", false, "running on CI")
var tc *docker.TestConfig
var snapshotname string

func Test_DockerToken(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Token Test Suite", suiteConfig, reporterConfig)
}

var _ = Describe("Use the token CLI to create and join agents", Ordered, func() {
	Context("Setup cluster with 3 servers", func() {
		It("should provision servers and agents", func() {
			var err error
			tc, err = docker.NewTestConfig("rancher/systemd-node")
			Expect(err).NotTo(HaveOccurred())
			Expect(tc.ProvisionServers(3)).To(Succeed())
			tc.SkipStart = true
			Expect(tc.ProvisionAgents(2)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(tc.KubeconfigFile)
			}, "60s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetServerNames())
			}, "40s", "5s").Should(Succeed())
			// Agents are opening alot of files, so expand the limit
			for _, node := range tc.Agents {
				_, err := node.RunCmdOnNode("sysctl -w fs.inotify.max_user_instances=8192")
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
	Context("Agent joins with permanent token:", func() {
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
			cmd := fmt.Sprintf("echo 'token: %s' | tee -a /etc/rancher/k3s/config.yaml > /dev/null", permToken)
			_, err := tc.Agents[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred())
			_, err = tc.Agents[0].RunCmdOnNode("systemctl start k3s-agent")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				nodes, err := tests.ParseNodes(tc.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).Should(Equal(len(tc.Servers) + 1))
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
			cmd := fmt.Sprintf("echo 'token: %s' | tee -a /etc/rancher/k3s/config.yaml > /dev/null", tempToken)
			_, err := tc.Agents[1].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred())
			_, err = tc.Agents[1].RunCmdOnNode("systemctl start k3s-agent")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				nodes, err := tests.ParseNodes(tc.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).Should(Equal(len(tc.Servers) + 2))
				g.Expect(tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())).Should(Succeed())
			}, "60s", "5s").Should(Succeed())
		})
	})
	Context("Rotate server bootstrap token", func() {
		serverToken := "1234"
		It("Creates a new server token", func() {
			cmd := fmt.Sprintf("k3s token rotate -t %s --new-token=%s", tc.Token, serverToken)
			Expect(tc.Servers[0].RunCmdOnNode(cmd)).
				To(ContainSubstring("Token rotated, restart k3s nodes with new token"))
		})
		It("Restarts servers with the new token", func() {
			cmd := fmt.Sprintf("echo 'token: %s' | tee -a /etc/rancher/k3s/config.yaml > /dev/null", serverToken)
			for _, node := range tc.Servers {
				_, err := node.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred())
			}
			for _, node := range tc.Servers {
				_, err := node.RunCmdOnNode("systemctl restart k3s")
				Expect(err).NotTo(HaveOccurred())
			}
			Eventually(func(g Gomega) {
				nodes, err := tests.ParseNodes(tc.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).Should(Equal(len(tc.Servers) + 2))
				g.Expect(tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())).Should(Succeed())
			}, "60s", "5s").Should(Succeed())
		})
		It("Rejoins an agent with the new server token", func() {
			cmd := fmt.Sprintf("sed -i 's/token:.*/token: %s/' /etc/rancher/k3s/config.yaml", serverToken)
			_, err := tc.Agents[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred())
			_, err = tc.Agents[0].RunCmdOnNode("systemctl restart k3s-agent")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				nodes, err := tests.ParseNodes(tc.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).Should(Equal(len(tc.Servers) + 2))
				g.Expect(tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())).Should(Succeed())
			}, "60s", "5s").Should(Succeed())
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if *ci || (tc != nil && !failed) {
		Expect(tc.Cleanup()).To(Succeed())
	}
})
