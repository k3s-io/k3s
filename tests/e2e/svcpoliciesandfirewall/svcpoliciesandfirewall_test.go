// This test verifies:
// * externaltrafficpolicy for both local and cluster values
// * internaltrafficpolicy for both local and cluster values
// * services firewall based on loadBalancerSourceRanges field

package svcpoliciesandfirewall

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"text/template"

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

func Test_E2EPoliciesAndFirewall(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Services Traffic Policies and Firewall config Suite", suiteConfig, reporterConfig)
}

var (
	tc    *e2e.TestConfig
	nodes []e2e.Node
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Services Traffic policies and firewall config", Ordered, func() {

	Context("Start cluster with minimal configuration", func() {
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
			Eventually(func(g Gomega) {
				var err error
				nodes, err = e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "300s", "5s").Should(Succeed())
			_, err := e2e.ParseNodes(tc.KubeconfigFile, true)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks Pod Status", func() {
			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "300s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})
	})
	Context("Deploy external traffic workloads to test external traffic policies", func() {
		// Verifies that the service with external traffic policy=local is deployed
		// Verifies that the external-ip is only set to the node IP where the server runs
		// It also verifies that the service with external traffic policy=cluster has both node IPs as externalIP
		It("Verify external traffic policy=local gets set up correctly", func() {
			_, err := tc.DeployWorkload("loadbalancer.yaml")
			Expect(err).NotTo(HaveOccurred(), "loadbalancer not deployed")
			_, err = tc.DeployWorkload("loadbalancer-extTrafficPol.yaml")
			Expect(err).NotTo(HaveOccurred(), "loadbalancer-extTrafficPol not deployed")

			// Check where the server pod is running
			var serverNodeName string
			Eventually(func() (string, error) {
				pods, err := tests.ParsePods(tc.KubeconfigFile)
				Expect(err).NotTo(HaveOccurred(), "failed to parse pods")
				for _, pod := range pods {
					if strings.Contains(pod.Name, "test-loadbalancer-ext") {
						serverNodeName = pod.Spec.NodeName
						break
					}
				}
				return serverNodeName, nil
			}, "25s", "5s").ShouldNot(BeEmpty(), "server pod not found")

			var serverNodeIP string
			for _, node := range nodes {
				if node.Name == serverNodeName {
					serverNodeIP = node.InternalIP
				}
			}

			// Verify there is only one external-ip and it is matching the node IP
			lbSvc := "nginx-loadbalancer-svc"
			lbSvcExt := "nginx-loadbalancer-svc-ext"
			Eventually(func() ([]string, error) {
				return e2e.FetchExternalIPs(tc.KubeconfigFile, lbSvc)
			}, "25s", "5s").Should(HaveLen(2), "external IP count not equal to 2")

			Eventually(func(g Gomega) {
				externalIPs, _ := e2e.FetchExternalIPs(tc.KubeconfigFile, lbSvcExt)
				g.Expect(externalIPs).To(HaveLen(1), "more than 1 exernalIP found")
				g.Expect(externalIPs[0]).To(Equal(serverNodeIP), "external IP does not match servernodeIP")
			}, "25s", "5s").Should(Succeed())
		})

		// Verifies that the service is reachable from the outside and the source IP is nos MASQ
		// It also verifies that the service with external traffic policy=cluster can be accessed and the source IP is MASQ
		It("Verify connectivity in external traffic policy=local", func() {
			lbSvc := "nginx-loadbalancer-svc"
			lbSvcExternalIPs, _ := e2e.FetchExternalIPs(tc.KubeconfigFile, lbSvc)
			lbSvcExt := "nginx-loadbalancer-svc-ext"
			lbSvcExtExternalIPs, _ := e2e.FetchExternalIPs(tc.KubeconfigFile, lbSvcExt)

			// Verify connectivity to the external IP of the lbsvc service and the IP should be the flannel interface IP because of MASQ
			for _, externalIP := range lbSvcExternalIPs {
				Eventually(func() (string, error) {
					cmd := "curl -m 5 -s -f http://" + externalIP + ":81/ip"
					return e2e.RunCommand(cmd)
				}, "25s", "5s").Should(ContainSubstring("10.42"))
			}

			// Verify connectivity to the external IP of the lbsvcExt service and the IP should not be the flannel interface IP
			Eventually(func() (string, error) {
				cmd := "curl -m 5 -s -f http://" + lbSvcExtExternalIPs[0] + ":82/ip"
				return e2e.RunCommand(cmd)
			}, "25s", "5s").ShouldNot(ContainSubstring("10.42"))

			// Verify connectivity to the other nodeIP does not work because of external traffic policy=local
			for _, externalIP := range lbSvcExternalIPs {
				if externalIP == lbSvcExtExternalIPs[0] {
					// This IP we already test and it shuold work
					continue
				}
				Eventually(func() error {
					cmd := "curl -m 5 -s -f http://" + externalIP + ":82/ip"
					_, err := e2e.RunCommand(cmd)
					return err
				}, "40s", "5s").Should(MatchError(ContainSubstring("exit status")))
			}
		})

		// Verifies that the internal traffic policy=local is deployed
		It("Verify internal traffic policy=local gets set up correctly", func() {
			_, err := tc.DeployWorkload("loadbalancer-intTrafficPol.yaml")
			Expect(err).NotTo(HaveOccurred(), "loadbalancer-intTrafficPol not deployed")
			_, err = tc.DeployWorkload("pod_client.yaml")
			Expect(err).NotTo(HaveOccurred(), "pod client not deployed")

			// Check that service exists
			Eventually(func() (string, error) {
				clusterIP, _ := e2e.FetchClusterIP(tc.KubeconfigFile, "nginx-loadbalancer-svc-int", false)
				return clusterIP, nil
			}, "25s", "5s").Should(ContainSubstring("10.43"))

			// Check that client pods are running
			Eventually(func() string {
				pods, err := tests.ParsePods(tc.KubeconfigFile)
				Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "client-deployment") {
						return string(pod.Status.Phase)
					}
				}
				return ""
			}, "50s", "5s").Should(Equal("Running"))
		})

		// Verifies that only the client pod running in the same node as the server pod can access the service
		// It also verifies that the service with internal traffic policy=cluster can be accessed by both client pods
		It("Verify connectivity in internal traffic policy=local", func() {
			var clientPod1, clientPod1Node, clientPod1IP, clientPod2, clientPod2Node, clientPod2IP, serverNodeName string
			Eventually(func(g Gomega) {
				pods, err := tests.ParsePods(tc.KubeconfigFile)
				Expect(err).NotTo(HaveOccurred(), "failed to parse pods")
				for _, pod := range pods {
					if strings.Contains(pod.Name, "test-loadbalancer-int") {
						serverNodeName = pod.Spec.NodeName
					}
					if strings.Contains(pod.Name, "client-deployment") {
						if clientPod1 == "" {
							clientPod1 = pod.Name
							clientPod1Node = pod.Spec.NodeName
							clientPod1IP = pod.Status.PodIP
						} else {
							clientPod2 = pod.Name
							clientPod2Node = pod.Spec.NodeName
							clientPod2IP = pod.Status.PodIP
						}
					}
				}
				// As we need those variables for the connectivity test, let's check they are not emtpy
				g.Expect(serverNodeName).ShouldNot(BeEmpty(), "server pod for internalTrafficPolicy=local not found")
				g.Expect(clientPod1).ShouldNot(BeEmpty(), "client pod1 not found")
				g.Expect(clientPod2).ShouldNot(BeEmpty(), "client pod2 not found")
				g.Expect(clientPod1Node).ShouldNot(BeEmpty(), "client pod1 node not found")
				g.Expect(clientPod2Node).ShouldNot(BeEmpty(), "client pod2 node not found")
				g.Expect(clientPod1IP).ShouldNot(BeEmpty(), "client pod1 IP not found")
				g.Expect(clientPod2IP).ShouldNot(BeEmpty(), "client pod2 IP not found")
			}, "25s", "5s").Should(Succeed(), "All pod and names and IPs should be non-empty")

			// Check that clientPod1Node and clientPod2Node are not equal
			Expect(clientPod1Node).ShouldNot(Equal(clientPod2Node))

			var workingCmd, nonWorkingCmd string
			if serverNodeName == clientPod1Node {
				workingCmd = "kubectl exec " + clientPod1 + " -- curl -m 5 -s -f http://nginx-loadbalancer-svc-int:83/ip"
				nonWorkingCmd = "kubectl exec " + clientPod2 + " -- curl -m 5 -s -f http://nginx-loadbalancer-svc-int:83/ip"
			}
			if serverNodeName == clientPod2Node {
				workingCmd = "kubectl exec " + clientPod2 + " -- curl -m 5 -s -f http://nginx-loadbalancer-svc-int:83/ip"
				nonWorkingCmd = "kubectl exec " + clientPod1 + " -- curl -m 5 -s -f http://nginx-loadbalancer-svc-int:83/ip"
			}

			Eventually(func() (string, error) {
				out, err := e2e.RunCommand(workingCmd)
				return out, err
			}, "25s", "5s").Should(SatisfyAny(
				ContainSubstring(clientPod1IP),
				ContainSubstring(clientPod2IP),
			))

			// Check the non working command fails because of internal traffic policy=local
			Eventually(func() bool {
				_, err := e2e.RunCommand(nonWorkingCmd)
				if err != nil && strings.Contains(err.Error(), "exit status") {
					// Treat exit status as a successful condition
					return true
				}
				return false
			}, "40s", "5s").Should(BeTrue())

			// curling a service with internal traffic policy=cluster. It should work on both pods
			for _, pod := range []string{clientPod1, clientPod2} {
				cmd := "kubectl exec " + pod + " -- curl -m 5 -s -f http://nginx-loadbalancer-svc:81/ip"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "20s", "5s").Should(SatisfyAny(
					ContainSubstring(clientPod1IP),
					ContainSubstring(clientPod2IP),
				))
			}
		})

		// Set up the service manifest with loadBalancerSourceRanges
		It("Applies service manifest with loadBalancerSourceRanges", func() {
			// Define the service manifest with a placeholder for the IP
			serviceManifest := `
apiVersion: v1
kind: Service
metadata:
name: nginx-loadbalancer-svc-ext-firewall
spec:
type: LoadBalancer
loadBalancerSourceRanges:
- {{.NodeIP}}/32
ports:
- port: 82
	targetPort: 80
	protocol: TCP
	name: http
selector:
	k8s-app: nginx-app-loadbalancer-ext
`
			// Remove the service nginx-loadbalancer-svc-ext
			_, err := e2e.RunCommand("kubectl delete svc nginx-loadbalancer-svc-ext")
			Expect(err).NotTo(HaveOccurred(), "failed to remove service nginx-loadbalancer-svc-ext")

			// Parse and execute the template with the node IP
			tmpl, err := template.New("service").Parse(serviceManifest)
			Expect(err).NotTo(HaveOccurred())

			var filledManifest strings.Builder
			err = tmpl.Execute(&filledManifest, struct{ NodeIP string }{NodeIP: nodes[0].InternalIP})
			Expect(err).NotTo(HaveOccurred())

			// Write the filled manifest to a temporary file
			tmpFile, err := os.CreateTemp("", "service-*.yaml")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(filledManifest.String())
			Expect(err).NotTo(HaveOccurred())
			tmpFile.Close()

			// Apply the manifest using kubectl
			applyCmd := fmt.Sprintf("kubectl apply -f %s", tmpFile.Name())
			out, err := e2e.RunCommand(applyCmd)
			Expect(err).NotTo(HaveOccurred(), out)

			Eventually(func() (string, error) {
				clusterIP, _ := e2e.FetchClusterIP(tc.KubeconfigFile, "nginx-loadbalancer-svc-ext-firewall", false)
				return clusterIP, nil
			}, "25s", "5s").Should(ContainSubstring("10.43"))
		})

		// Verify that only the allowed node can curl. That node should be able to curl both externalIPs (i.e. node.InternalIP)
		It("Verify firewall is working", func() {
			for _, node := range nodes {
				var sNode, aNode e2e.VagrantNode
				for _, n := range tc.Servers {
					if n.String() == nodes[0].Name {
						sNode = n
					}
				}
				for _, n := range tc.Agents {
					if n.String() == nodes[1].Name {
						aNode = n
					}
				}

				// Verify connectivity from nodes[0] works because we passed its IP to the loadBalancerSourceRanges
				Eventually(func() (string, error) {
					cmd := "curl -m 5 -s -f http://" + node.InternalIP + ":82"
					return sNode.RunCmdOnNode(cmd)
				}, "40s", "5s").Should(ContainSubstring("Welcome to nginx"))

				// Verify connectivity from nodes[1] fails because we did not pass its IP to the loadBalancerSourceRanges
				Eventually(func(g Gomega) error {
					cmd := "curl -m 5 -s -f http:// " + node.InternalIP + ":82"
					_, err := aNode.RunCmdOnNode(cmd)
					return err
				}, "40s", "5s").Should(MatchError(ContainSubstring("exit status")))
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
		AddReportEntry("journald-logs", e2e.TailJournalLogs(1000, append(tc.Servers, tc.Agents...)))
	} else {
		Expect(e2e.GetCoverageReport(append(tc.Servers, tc.Agents...))).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
