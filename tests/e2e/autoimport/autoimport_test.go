package autoimport

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

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

var _ = ReportAfterEach(e2e.GenReport)

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
			}, "620s", "5s").Should(Succeed())
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
			}, "620s", "5s").Should(Succeed())
			_, _ = e2e.ParsePods(kubeConfigFile, true)
		})

		It("Create a folder in agent/images", func() {
			cmd := `mkdir /var/lib/rancher/k3s/agent/images`
			_, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)
		})

		It("Create file for auto import and search in the image store", func() {
			cmd := `echo docker.io/library/redis:latest | sudo tee /var/lib/rancher/k3s/agent/images/testautoimport.txt`
			_, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/redis`
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

		It("Change name for the file and see if the label is still pinned", func() {
			cmd := `mv /var/lib/rancher/k3s/agent/images/testautoimport.txt /var/lib/rancher/k3s/agent/images/testautoimportrename.txt`
			_, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/redis`
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

		It("Create, remove and create again a file", func() {
			cmd := `echo docker.io/library/busybox:latest | sudo tee /var/lib/rancher/k3s/agent/images/bb.txt`
			_, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/busybox`
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())

			cmd = `rm /var/lib/rancher/k3s/agent/images/bb.txt`
			_, err = e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/busybox`
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())

			cmd = `echo docker.io/library/busybox:latest | sudo tee /var/lib/rancher/k3s/agent/images/bb.txt`
			_, err = e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/busybox`
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

		It("Move the folder, add a image and then see if the image is going to be pinned", func() {
			cmd := `mv /var/lib/rancher/k3s/agent/images /var/lib/rancher/k3s/agent/test`
			_, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			cmd = `echo 'docker.io/library/mysql:latest' | sudo tee /var/lib/rancher/k3s/agent/test/mysql.txt`
			_, err = e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			cmd = `mv /var/lib/rancher/k3s/agent/test /var/lib/rancher/k3s/agent/images`
			_, err = e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/mysql`
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

		It("Restarts normally", func() {
			errRestart := e2e.RestartCluster(append(serverNodeNames, agentNodeNames...))
			Expect(errRestart).NotTo(HaveOccurred(), "Restart Nodes not happened correctly")

			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
		})

		It("Verify bb.txt image and see if are pinned", func() {
			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/busybox`
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

		It("Removes bb.txt file", func() {
			cmd := `rm /var/lib/rancher/k3s/agent/images/bb.txt`
			_, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/busybox`
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Should(ContainSubstring("io.cri-containerd.pinned=pinned"))
			}, "620s", "5s").Should(Succeed())
		})

		It("Restarts normally", func() {
			errRestart := e2e.RestartCluster(append(serverNodeNames, agentNodeNames...))
			Expect(errRestart).NotTo(HaveOccurred(), "Restart Nodes not happened correctly")

			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
		})

		It("Verify if bb.txt image is unpinned", func() {
			Eventually(func(g Gomega) {
				cmd := `k3s ctr images list | grep library/busybox`
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).ShouldNot(ContainSubstring("io.cattle.k3s.pinned=pinned"))
				g.Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).ShouldNot(ContainSubstring("io.cri-containerd.pinned=pinned"))
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
		Expect(e2e.GetCoverageReport(append(serverNodeNames, agentNodeNames...))).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
