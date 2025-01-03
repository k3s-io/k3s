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

var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 or nil for latest commit from master

func Test_E2EBtrfsSnapshot(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Btrfs Snapshot Test Suite", suiteConfig, reporterConfig)
}

var (
	kubeConfigFile  string
	serverNodeNames []string
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify that btrfs based servers work", Ordered, func() {
	Context("Btrfs Snapshots are taken", func() {
		It("Starts up with no issues", func() {
			var err error
			// OS and server are hardcoded because only openSUSE Leap 15.5 natively supports Btrfs
			if *local {
				serverNodeNames, _, err = e2e.CreateLocalCluster("opensuse/Leap-15.6.x86_64", 1, 0)
			} else {
				serverNodeNames, _, err = e2e.CreateCluster("opensuse/Leap-15.6.x86_64", 1, 0)
			}
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			fmt.Println("CLUSTER CONFIG")
			fmt.Println("Server Nodes:", serverNodeNames)
			kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(kubeConfigFile, true)

			fmt.Printf("\nFetching pods status\n")
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
		It("Checks that btrfs snapshots exist", func() {
			cmd := "btrfs subvolume list /var/lib/rancher/k3s/agent/containerd/io.containerd.snapshotter.v1.btrfs"
			res, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(MatchRegexp("agent/containerd/io.containerd.snapshotter.v1.btrfs/active/\\d+"))
			Expect(res).To(MatchRegexp("agent/containerd/io.containerd.snapshotter.v1.btrfs/snapshots/\\d+"))
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		Expect(e2e.SaveJournalLogs(serverNodeNames)).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
