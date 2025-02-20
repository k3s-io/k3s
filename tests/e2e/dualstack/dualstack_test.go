package dualstack

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var hardened = flag.Bool("hardened", false, "true or false")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

func Test_E2EDualStack(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "DualStack Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify DualStack Configuration", Ordered, func() {
	Context("Cluster Deploys with both IPv6 and IPv4 networks", func() {
		It("Starts up with no issues", func() {
			var err error
			if *local {
				tc, err = e2e.CreateLocalCluster(*nodeOS, *serverCount, *agentCount)
			} else {
				tc, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
			}
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			tc.Hardened = *hardened
			By("CLUSTER CONFIG")
			By("OS: " + *nodeOS)
			By(tc.Status())
		})

		It("Checks Node Status", func() {
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
			_, err := e2e.ParseNodes(tc.KubeconfigFile, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks pod status", func() {
			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "620s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})

		It("Verifies that each node has IPv4 and IPv6", func() {
			nodeIPs, err := e2e.GetNodeIPs(tc.KubeconfigFile)
			Expect(err).NotTo(HaveOccurred())
			for _, node := range nodeIPs {
				Expect(node.IPv4).Should(ContainSubstring("10.10.10"))
				Expect(node.IPv6).Should(ContainSubstring("fd11:decf:c0ff"))
			}
		})
		It("Verifies that each pod has IPv4 and IPv6", func() {
			podIPs, err := e2e.GetPodIPs(tc.KubeconfigFile)
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range podIPs {
				Expect(pod.IPv4).Should(Or(ContainSubstring("10.10.10"), ContainSubstring("10.42.")), pod.Name)
				Expect(pod.IPv6).Should(Or(ContainSubstring("fd11:decf:c0ff"), ContainSubstring("2001:cafe:42")), pod.Name)
			}
		})

		It("Verifies ClusterIP Service", func() {
			_, err := tc.DeployWorkload("dualstack_clusterip.yaml")
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() (string, error) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				return e2e.RunCommand(cmd)
			}, "120s", "5s").Should(ContainSubstring("ds-clusterip-pod"))

			// Checks both IPv4 and IPv6
			clusterips, err := e2e.FetchClusterIP(tc.KubeconfigFile, "ds-clusterip-svc", true)
			Expect(err).NotTo(HaveOccurred())
			for _, ip := range strings.Split(clusterips, ",") {
				if strings.Contains(ip, "::") {
					ip = "[" + ip + "]"
				}
				pods, err := tests.ParsePods(tc.KubeconfigFile)
				Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if !strings.HasPrefix(pod.Name, "ds-clusterip-pod") {
						continue
					}
					cmd := fmt.Sprintf("curl -m 5 -s -f http://%s", ip)
					Eventually(func() (string, error) {
						return tc.Servers[0].RunCmdOnNode(cmd)
					}, "60s", "5s").Should(ContainSubstring("Welcome to nginx!"), "failed cmd: "+cmd)
				}
			}
		})
		It("Verifies Ingress", func() {
			_, err := tc.DeployWorkload("dualstack_ingress.yaml")
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")
			cmd := "kubectl get ingress ds-ingress -o jsonpath=\"{.spec.rules[*].host}\""
			hostName, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
			nodeIPs, err := e2e.GetNodeIPs(tc.KubeconfigFile)
			Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
			for _, node := range nodeIPs {
				cmd := fmt.Sprintf("curl --header host:%s -m 5 -s -f http://%s/name.html", hostName, node.IPv4)
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "10s", "2s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
				cmd = fmt.Sprintf("curl --header host:%s -m 5 -s -f http://[%s]/name.html", hostName, node.IPv6)
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "5s", "1s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
			}
		})

		It("Verifies NodePort Service", func() {
			_, err := tc.DeployWorkload("dualstack_nodeport.yaml")
			Expect(err).NotTo(HaveOccurred())
			cmd := "kubectl get service ds-nodeport-svc --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
			nodeIPs, err := e2e.GetNodeIPs(tc.KubeconfigFile)
			Expect(err).NotTo(HaveOccurred())
			for _, node := range nodeIPs {
				cmd = "curl -m 5 -s -f http://" + node.IPv4 + ":" + nodeport + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "10s", "1s").Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
				cmd = "curl -m 5 -s -f http://[" + node.IPv6 + "]:" + nodeport + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "10s", "1s").Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
			}
		})
		It("Verifies podSelector Network Policy", func() {
			_, err := tc.DeployWorkload("pod_client.yaml")
			Expect(err).NotTo(HaveOccurred())
			cmd := "kubectl exec svc/client-curl -- curl -m 5 -s -f http://ds-clusterip-svc/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "20s", "3s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
			_, err = tc.DeployWorkload("netpol-fail.yaml")
			Expect(err).NotTo(HaveOccurred())
			cmd = "kubectl exec svc/client-curl -- curl -m 5 -s -f http://ds-clusterip-svc/name.html"
			Eventually(func() error {
				_, err = e2e.RunCommand(cmd)
				Expect(err).To(HaveOccurred())
				return err
			}, "20s", "3s")
			_, err = tc.DeployWorkload("netpol-work.yaml")
			Expect(err).NotTo(HaveOccurred())
			cmd = "kubectl exec svc/client-curl -- curl -m 5 -s -f http://ds-clusterip-svc/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "20s", "3s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		AddReportEntry("journald-logs", e2e.TailJournalLogs(1000, append(tc.Servers, tc.Agents...)))
	} else {
		Expect(e2e.GetCoverageReport(append(tc.Servers, tc.Agents...))).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
