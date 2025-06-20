package etcdmigration

import (
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 or nil for latest commit from master

// This test suite is used to verify that K3s can migrate from SQLite to etcd datastore
// without losing data or functionality.

func Test_E2EEtcdMigration(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	suiteConfig.Timeout = 30 * time.Minute
	RunSpecs(t, "Etcd Migration Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

// StartK3sWithSQLite starts K3s with SQLite as the datastore
func StartK3sWithSQLite(nodes []e2e.VagrantNode) error {
	for _, node := range nodes {
		var startCmd string
		if strings.Contains(node.String(), "server") {
			startCmd = "systemctl start k3s"
		} else {
			startCmd = "systemctl start k3s-agent"
		}
		if _, err := node.RunCmdOnNode(startCmd); err != nil {
			return &e2e.NodeError{Node: node, Cmd: startCmd, Err: err}
		}
	}
	return nil
}

// MigrateToEtcd migrates the K3s datastore from SQLite to etcd
func MigrateToEtcd(server e2e.VagrantNode) error {
	// Stop K3s service
	stopCmd := "systemctl stop k3s"
	if _, err := server.RunCmdOnNode(stopCmd); err != nil {
		return &e2e.NodeError{Node: server, Cmd: stopCmd, Err: err}
	}

	// Update config to use etcd
	configCmd := "echo 'cluster-init: true' >> /etc/rancher/k3s/config.yaml"
	if _, err := server.RunCmdOnNode(configCmd); err != nil {
		return &e2e.NodeError{Node: server, Cmd: configCmd, Err: err}
	}

	// Start K3s with etcd
	startCmd := "systemctl start k3s"
	if _, err := server.RunCmdOnNode(startCmd); err != nil {
		return &e2e.NodeError{Node: server, Cmd: startCmd, Err: err}
	}

	return nil
}

// KillK3sCluster kills the K3s cluster
func KillK3sCluster(nodes []e2e.VagrantNode) error {
	for _, node := range nodes {
		if _, err := node.RunCmdOnNode("k3s-killall.sh"); err != nil {
			return err
		}
		if _, err := node.RunCmdOnNode("rm -rf /etc/rancher/k3s/config.yaml.d /var/lib/kubelet/pods /var/lib/rancher/k3s/agent/etc /var/lib/rancher/k3s/agent/containerd /var/lib/rancher/k3s/server/db /var/log/pods /run/k3s /run/flannel"); err != nil {
			return err
		}
		if _, err := node.RunCmdOnNode("systemctl restart containerd"); err != nil {
			return err
		}
		if _, err := node.RunCmdOnNode("journalctl --flush --sync --rotate --vacuum-size=1"); err != nil {
			return err
		}
	}
	return nil
}

var _ = ReportAfterEach(e2e.GenReport)

var _ = BeforeSuite(func() {
	var err error
	if *local {
		tc, err = e2e.CreateLocalCluster(*nodeOS, 1, 1)
	} else {
		tc, err = e2e.CreateCluster(*nodeOS, 1, 1)
	}
	Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
})

var _ = Describe("Etcd Migration", Ordered, func() {
	Context("Verify SQLite to Etcd migration", func() {
		It("Starts K3s with SQLite datastore", func() {
			err := StartK3sWithSQLite(tc.AllNodes())
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))

			By("CLUSTER CONFIG")
			By("OS:" + *nodeOS)
			By(tc.Status())
			tc.KubeconfigFile, err = e2e.GenKubeconfigFile(tc.Servers[0].String())
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status with SQLite", func() {
			By("Fetching node status")
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, e2e.VagrantSlice(tc.AllNodes()))
			}, "600s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "600s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(tc.KubeconfigFile)
			}, "480s", "10s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})

		It("Creates test resources before migration", func() {
			// Create a test ConfigMap to verify data persistence
			createCmd := "kubectl create configmap migration-test --from-literal=test=before-migration"
			_, err := tc.Servers[0].RunCmdOnNode(createCmd)
			Expect(err).NotTo(HaveOccurred())

			// Verify the ConfigMap exists
			getCmd := "kubectl get configmap migration-test -o jsonpath='{.data.test}'"
			result, err := tc.Servers[0].RunCmdOnNode(getCmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("before-migration"))
		})

		It("Migrates from SQLite to etcd", func() {
			err := MigrateToEtcd(tc.Servers[0])
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))

			// Verify etcd is running
			Eventually(func() (string, error) {
				return tc.Servers[0].RunCmdOnNode("systemctl status k3s | grep 'Started k3s'")
			}, "60s", "5s").Should(ContainSubstring("Started k3s"))

			// Verify etcd is being used
			Eventually(func() (string, error) {
				return tc.Servers[0].RunCmdOnNode("journalctl -u k3s --no-pager | grep -i etcd")
			}, "120s", "5s").Should(ContainSubstring("etcd"))
		})

		It("Checks node and pod status after migration", func() {
			By("Fetching node status after migration")
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, e2e.VagrantSlice(tc.AllNodes()))
			}, "600s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "600s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(tc.KubeconfigFile)
			}, "480s", "10s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})

		It("Verifies data persistence after migration", func() {
			// Verify the ConfigMap still exists with the same data
			getCmd := "kubectl get configmap migration-test -o jsonpath='{.data.test}'"
			result, err := tc.Servers[0].RunCmdOnNode(getCmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("before-migration"))
		})

		It("Kills the cluster", func() {
			err := KillK3sCluster(tc.AllNodes())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if tc == nil {
		return
	}
	if failed {
		AddReportEntry("config", e2e.GetConfig(tc.AllNodes()))
		AddReportEntry("pods", e2e.DescribePods(tc.KubeconfigFile))
		Expect(e2e.SaveJournalLogs(tc.AllNodes())).To(Succeed())
		Expect(e2e.TailPodLogs(50, tc.AllNodes())).To(Succeed())
		Expect(e2e.SaveNetwork(tc.AllNodes())).To(Succeed())
		Expect(e2e.SaveKernel(tc.AllNodes())).To(Succeed())
	} else {
		Expect(e2e.GetCoverageReport(tc.AllNodes())).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
