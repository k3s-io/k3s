package autoimport

import (
	"flag"
	"os"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS:
// bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
// eurolinux-vagrant/rocky-8, eurolinux-vagrant/rocky-9,
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 1, "number of server nodes")
var agentCount = flag.Int("agentCount", 0, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 (default: latest commit from master)
// E2E_REGISTRY: true/false (default: false)

func Test_E2EAutoImport(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Create Cluster Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Create", Ordered, func() {
	Context("Cluster :", func() {
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

		It("Checks node and pod status", func() {
			By("Fetching Nodes status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeConfigFile)

			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeConfigFile)
			}, "620s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeConfigFile)
		})

		It("Create a folder in agent/images", func() {
			cmd := `mkdir /var/lib/rancher/k3s/agent/images`
			_, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)
		})

		It("Create file for auto import and search in the image store", func() {
			cmd := `echo docker.io/library/redis:latest | sudo tee /var/lib/rancher/k3s/agent/images/testautoimport.txt`
			_, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/redis`
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

		It("Change name for the file and see if the label is still pinned", func() {
			cmd := `mv /var/lib/rancher/k3s/agent/images/testautoimport.txt /var/lib/rancher/k3s/agent/images/testautoimportrename.txt`
			_, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/redis`
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

		It("Create, remove and create again a file", func() {
			cmd := `echo docker.io/library/busybox:latest | sudo tee /var/lib/rancher/k3s/agent/images/bb.txt`
			_, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/busybox`
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())

			cmd = `rm /var/lib/rancher/k3s/agent/images/bb.txt`
			_, err = tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/busybox`
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())

			cmd = `echo docker.io/library/busybox:latest | sudo tee /var/lib/rancher/k3s/agent/images/bb.txt`
			_, err = tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/busybox`
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

		It("Move the folder, add a image and then see if the image is going to be pinned", func() {
			cmd := `mv /var/lib/rancher/k3s/agent/images /var/lib/rancher/k3s/agent/test`
			_, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			cmd = `echo 'docker.io/library/mysql:latest' | sudo tee /var/lib/rancher/k3s/agent/test/mysql.txt`
			_, err = tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			cmd = `mv /var/lib/rancher/k3s/agent/test /var/lib/rancher/k3s/agent/images`
			_, err = tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/mysql`
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

		It("Restarts normally", func() {
			errRestart := e2e.RestartCluster(append(tc.Servers, tc.Agents...))
			Expect(errRestart).NotTo(HaveOccurred(), "Restart Nodes not happened correctly")

			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
		})

		It("Verify bb.txt image and see if are pinned", func() {
			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/busybox`
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

		It("Removes bb.txt file", func() {
			cmd := `rm /var/lib/rancher/k3s/agent/images/bb.txt`
			_, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/busybox`
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

		It("Restarts normally", func() {
			errRestart := e2e.RestartCluster(append(tc.Servers, tc.Agents...))
			Expect(errRestart).NotTo(HaveOccurred(), "Restart Nodes not happened correctly")

			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
		})

		It("Verify if bb.txt image is unpinned", func() {
			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/busybox`
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).ShouldNot(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(tc.Servers[0].RunCmdOnNode(cmd)).ShouldNot(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {

	if !failed {
		Expect(e2e.GetCoverageReport(append(tc.Servers, tc.Agents...))).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeConfigFile)).To(Succeed())
	}
})
