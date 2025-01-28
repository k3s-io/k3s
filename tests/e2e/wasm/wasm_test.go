package wasm

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

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 1, "number of server nodes")
var agentCount = flag.Int("agentCount", 0, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

func Test_E2EWasm(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Run WebAssenbly Workloads Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Can run Wasm workloads", Ordered, func() {

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

	// Server node needs to be ready before we continue
	It("Checks Node and Pod Status", func() {
		By("Fetching Nodes status")
		Eventually(func(g Gomega) {
			nodes, err := e2e.ParseNodes(tc.KubeConfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, node := range nodes {
				g.Expect(node.Status).Should(Equal("Ready"))
			}
		}, "620s", "5s").Should(Succeed())
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
		}, "620s", "5s").Should(Succeed())
		e2e.DumpPods(tc.KubeConfigFile)
	})

	It("Verify wasm-related containerd shims are installed", func() {
		expected_shims := []string{"containerd-shim-spin-v2", "containerd-shim-slight-v1"}
		for _, node := range append(tc.Servers, tc.Agents...) {
			for _, shim := range expected_shims {
				cmd := fmt.Sprintf("which %s", shim)
				_, err := node.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

	Context("Verify Wasm workloads can run on the cluster", func() {
		It("Deploy Wasm workloads", func() {
			out, err := tc.DeployWorkload("wasm-workloads.yaml")
			Expect(err).NotTo(HaveOccurred(), out)
		})

		It("Wait for slight Pod to be up and running", func() {
			Eventually(func() (string, error) {
				cmd := "kubectl get pods -o=name -l app=wasm-slight --field-selector=status.phase=Running --kubeconfig=" + tc.KubeConfigFile
				return e2e.RunCommand(cmd)
			}, "240s", "5s").Should(ContainSubstring("pod/wasm-slight"))
		})

		It("Wait for spin Pod to be up and running", func() {
			Eventually(func() (string, error) {
				cmd := "kubectl get pods -o=name -l app=wasm-spin --field-selector=status.phase=Running --kubeconfig=" + tc.KubeConfigFile
				return e2e.RunCommand(cmd)
			}, "120s", "5s").Should(ContainSubstring("pod/wasm-spin"))
		})

		It("Interact with Wasm applications", func() {
			ingressIPs, err := e2e.FetchIngressIP(tc.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(ingressIPs).To(HaveLen(1))

			endpoints := []string{"slight/hello", "spin/go-hello", "spin/hello"}
			for _, endpoint := range endpoints {
				url := fmt.Sprintf("http://%s/%s", ingressIPs[0], endpoint)
				fmt.Printf("Connecting to Wasm web application at: %s\n", url)
				cmd := "curl -sfv " + url

				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "120s", "5s").Should(ContainSubstring("200 OK"))
			}
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		Expect(e2e.SaveJournalLogs(append(tc.Servers, tc.Agents...))).To(Succeed())
	} else {
		Expect(e2e.GetCoverageReport(append(tc.Servers, tc.Agents...))).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeConfigFile)).To(Succeed())
	}
})
