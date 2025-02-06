package rootless

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Rootless is only valid on a single node, but requires node/kernel configuration, requiring a E2E test environment.

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.27.1+k3s2 or nil for latest commit from master

func Test_E2ERootless(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Rootless Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

func StartK3sCluster(nodes []e2e.VagrantNode, serverYAML string) error {
	for _, node := range nodes {

		resetCmd := "head -n 3 /etc/rancher/k3s/config.yaml > /tmp/config.yaml && sudo mv /tmp/config.yaml /etc/rancher/k3s/config.yaml"
		yamlCmd := fmt.Sprintf("echo '%s' >> /etc/rancher/k3s/config.yaml", serverYAML)
		startCmd := "systemctl --user restart k3s-rootless"

		if _, err := node.RunCmdOnNode(resetCmd); err != nil {
			return err
		}
		if _, err := node.RunCmdOnNode(yamlCmd); err != nil {
			return err
		}
		if _, err := RunCmdOnRootlessNode("systemctl --user daemon-reload", node.String()); err != nil {
			return err
		}
		if _, err := RunCmdOnRootlessNode(startCmd, node.String()); err != nil {
			return err
		}
	}
	return nil
}

func KillK3sCluster(nodes []e2e.VagrantNode) error {
	for _, node := range nodes {
		if _, err := RunCmdOnRootlessNode(`systemctl --user stop k3s-rootless`, node.String()); err != nil {
			return err
		}
		if _, err := RunCmdOnRootlessNode("k3s-killall.sh", node.String()); err != nil {
			return err
		}
		if _, err := RunCmdOnRootlessNode("rm -rf /home/vagrant/.rancher/k3s/server/db", node.String()); err != nil {
			return err
		}
	}
	return nil
}

var _ = ReportAfterEach(e2e.GenReport)

var _ = BeforeSuite(func() {
	var err error
	if *local {
		tc, err = e2e.CreateLocalCluster(*nodeOS, 1, 0)
	} else {
		tc, err = e2e.CreateCluster(*nodeOS, 1, 0)
	}
	Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
	//Checks if system is using cgroup v2
	_, err = tc.Servers[0].RunCmdOnNode("cat /sys/fs/cgroup/cgroup.controllers")
	Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))

})

var _ = Describe("Various Startup Configurations", Ordered, func() {
	Context("Verify standard startup :", func() {
		It("Starts K3s with no issues", func() {
			err := StartK3sCluster(tc.Servers, "")
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))

			By("CLUSTER CONFIG")
			By("OS: " + *nodeOS)
			By(tc.Status())
			kubeConfigFile, err := GenRootlessKubeconfigFile(tc.Servers[0].String())
			Expect(err).NotTo(HaveOccurred())
			tc.KubeconfigFile = kubeConfigFile
		})

		It("Checks node and pod status", func() {
			By("Fetching Nodes status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "360s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(tc.KubeconfigFile, false)

			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "360s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})

		It("Returns pod metrics", func() {
			cmd := "kubectl top pod -A"
			Eventually(func() error {
				_, err := e2e.RunCommand(cmd)
				return err
			}, "600s", "5s").Should(Succeed())
		})

		It("Returns node metrics", func() {
			cmd := "kubectl top node"
			_, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Runs an interactive command a pod", func() {
			cmd := "kubectl run busybox --rm -it --restart=Never --image=rancher/mirrored-library-busybox:1.34.1 -- uname -a"
			_, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Collects logs from a pod", func() {
			cmd := "kubectl logs -n kube-system -l app.kubernetes.io/name=traefik -c traefik"
			_, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Kills the cluster", func() {
			err := KillK3sCluster(tc.Servers)
			Expect(err).NotTo(HaveOccurred())
		})
	})

})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		AddReportEntry("journald-logs", e2e.TailJournalLogs(1000, tc.Servers))
	} else {
		Expect(e2e.GetCoverageReport(tc.Servers)).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
