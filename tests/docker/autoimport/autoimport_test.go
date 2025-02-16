package autoimport

import (
	"flag"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var ci = flag.Bool("ci", false, "running on CI")

func Test_DockerAutoImport(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Auto Import Test Suite", suiteConfig, reporterConfig)
}

var tc *docker.TestConfig

var _ = Describe("Verify Create", Ordered, func() {
	Context("Setup Cluster", func() {
		It("should provision server", func() {
			var err error
			tc, err = docker.NewTestConfig("rancher/systemd-node")
			Expect(err).NotTo(HaveOccurred())
			Expect(tc.ProvisionServers(1)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server", "traefik"}, tc.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})

	Context("Add images that should be imported by containerd automatically", func() {
		It("Create a folder in agent/images", func() {
			cmd := `mkdir /var/lib/rancher/k3s/agent/images`
			_, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)
		})

		It("Create file for auto import and search in the image store", func() {
			cmd := `echo docker.io/library/redis:latest | tee /var/lib/rancher/k3s/agent/images/testautoimport.txt`
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
			cmd := `echo docker.io/library/busybox:latest | tee /var/lib/rancher/k3s/agent/images/bb.txt`
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

			cmd = `echo docker.io/library/busybox:latest | tee /var/lib/rancher/k3s/agent/images/bb.txt`
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
			Expect(docker.RestartCluster(tc.Servers)).To(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())
			}, "60s", "5s").Should(Succeed())
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
			Expect(docker.RestartCluster(tc.Servers)).To(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())
			}, "60s", "5s").Should(Succeed())
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
	if *ci || (tc != nil && !failed) {
		Expect(tc.Cleanup()).To(Succeed())
	}
})
