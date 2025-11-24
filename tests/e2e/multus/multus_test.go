// This test verifies that two nodes, which can't connect using the local network, are
// able to still connect using the node-external-ip. In real life, node-external-ip
// would be a public IP. In the test, we create two networks, one sets the node
// internal-ip and the other sets the node-external-ip. Traffic is blocked on the former

package multus

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 1, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

func Test_E2EMultus(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Multus config Suite", suiteConfig, reporterConfig)

}

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Multus config", Ordered, func() {
	Context("Cluster comes up with Multus enabled", func() {
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

		It("Checks Node Status", func() {
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, e2e.VagrantSlice(tc.AllNodes()))
			}, "620s", "5s").Should(Succeed())
			e2e.DumpNodes(tc.KubeconfigFile)
		})

		It("Checks pod status", func() {
			By("Fetching pod status")
			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile, "kube-system")
			}, "620s", "10s").Should(Succeed())
		})
	})
	Context("Deploy workloads to check cluster connectivity of the nodes", func() {
		It("Verifies that each node has vagrant IP", func() {
			nodeIPs, err := e2e.GetNodeIPs(tc.KubeconfigFile)
			fmt.Printf("nodeIPs: %v", nodeIPs)
			Expect(err).NotTo(HaveOccurred())
			for _, node := range nodeIPs {
				Expect(node.IPv4).Should(ContainSubstring("10.10."))
			}
		})
		It("Verifies that each pod has vagrant IP or clusterCIDR IP", func() {
			podIPs, err := e2e.GetPodIPs(tc.KubeconfigFile)
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range podIPs {
				Expect(pod.IPv4).Should(Or(ContainSubstring("10.10."), ContainSubstring("10.42.")), pod.Name)
			}
		})
		It("Verifies multus daemonset comes up", func() {
			Eventually(func() (string, error) {
				cmd := "kubectl get ds multus -n kube-system -o jsonpath='{.status.numberReady}' --kubeconfig=" + tc.KubeconfigFile
				return e2e.RunCommand(cmd)
			}, "120s", "5s").Should(ContainSubstring("2"))
		})
		It("Deploys Multus NetworkAttachmentDefinition and test pods", func() {
			_, err := tc.DeployWorkload("multus_test.yaml")
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(5 * time.Second)
		})
		It("Verifies internode connectivity over multus network", func() {
			cmd := "kubectl exec pod-macvlan --kubeconfig=" + tc.KubeconfigFile + " -- ping -c 1 -w 2 10.1.1.102"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "20s", "3s").Should(ContainSubstring("0% packet loss"), "failed cmd: "+cmd)
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		Expect(e2e.SaveJournalLogs(tc.AllNodes())).To(Succeed())
		Expect(e2e.TailPodLogs(50, tc.AllNodes())).To(Succeed())
	} else {
		Expect(e2e.GetCoverageReport(tc.AllNodes())).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
