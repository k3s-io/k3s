package btrfs

import (
	"flag"
	"os"
	"testing"

	"github.com/k3s-io/k3s/tests"
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

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify that btrfs based servers work", Ordered, func() {
	Context("Btrfs Snapshots are taken", func() {
		It("Starts up with no issues", func() {
			var err error
			// OS and server are hardcoded because only openSUSE Leap 15.5 natively supports Btrfs
			if *local {
				tc, err = e2e.CreateLocalCluster("opensuse/Leap-15.6.x86_64", 1, 0)
			} else {
				tc, err = e2e.CreateCluster("opensuse/Leap-15.6.x86_64", 1, 0)
			}
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			By("CLUSTER CONFIG")
			By(tc.Status())
		})

		It("Checks node and pod status", func() {
			By("Fetching Nodes status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)

			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "620s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})
		It("Checks that btrfs snapshots exist", func() {
			cmd := "btrfs subvolume list /var/lib/rancher/k3s/agent/containerd/io.containerd.snapshotter.v1.btrfs"
			Eventually(func(g Gomega) {
				res, err := tc.Servers[0].RunCmdOnNode(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).To(MatchRegexp("agent/containerd/io.containerd.snapshotter.v1.btrfs/active/\\d+"))
				g.Expect(res).To(MatchRegexp("agent/containerd/io.containerd.snapshotter.v1.btrfs/snapshots/\\d+"))
			}, "30s", "5s").Should(Succeed())
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		Expect(e2e.SaveJournalLogs(tc.Servers)).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
