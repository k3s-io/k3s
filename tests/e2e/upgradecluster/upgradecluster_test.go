package upgradecluster

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

// Valid nodeOS:
// bento/ubuntu-24.04, eurolinux-vagrant/rocky-8, eurolinux-vagrant/rocky-9
// opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 2, "number of agent nodes")
var hardened = flag.Bool("hardened", false, "true or false")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "Controls which version k3s upgrades too, local binary or latest commit on master")

// Environment Variables Info:
// E2E_REGISTRY: true/false (default: false)
// Controls which K3s version is installed first
// E2E_RELEASE_VERSION=v1.23.3+k3s1
// OR
// E2E_RELEASE_CHANNEL=(commit|latest|stable), commit pulls latest commit from master

func Test_E2EUpgradeValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Upgrade Cluster Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Upgrade", Ordered, func() {
	Context("Cluster :", func() {
		It("Starts up with no issues", func() {
			var err error
			tc, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			tc.Hardened = *hardened
			By("CLUSTER CONFIG")
			By("OS: " + *nodeOS)
			By(tc.Status())
			Expect(err).NotTo(HaveOccurred())
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

			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "620s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})

		It("Verifies ClusterIP Service", func() {
			_, err := tc.DeployWorkload("clusterip.yaml")

			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")

			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "240s", "5s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)

			clusterip, _ := e2e.FetchClusterIP(tc.KubeconfigFile, "nginx-clusterip-svc", false)
			cmd = "curl -m 5 -s -f http://" + clusterip + "/name.html"
			for _, node := range tc.Servers {
				Eventually(func() (string, error) {
					return node.RunCmdOnNode(cmd)
				}, "120s", "10s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)
			}
		})

		It("Verifies NodePort Service", func() {
			_, err := tc.DeployWorkload("nodeport.yaml")
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")

			for _, node := range tc.Servers {
				nodeExternalIP, _ := node.FetchNodeExternalIP()
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)

				cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")

				cmd = "curl -m 5 -s -f http://" + nodeExternalIP + ":" + nodeport + "/name.html"
				fmt.Println(cmd)
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "failed cmd: "+cmd)
			}
		})

		It("Verifies LoadBalancer Service", func() {
			_, err := tc.DeployWorkload("loadbalancer.yaml")
			Expect(err).NotTo(HaveOccurred(), "Loadbalancer manifest not deployed")
			for _, node := range tc.Servers {
				ip, _ := node.FetchNodeExternalIP()
				cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].port}\""
				port, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"))

				cmd = "curl -m 5 -s -f http://" + ip + ":" + port + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"), "failed cmd: "+cmd)
			}
		})

		It("Verifies Ingress", func() {
			_, err := tc.DeployWorkload("ingress.yaml")
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")

			for _, node := range tc.Servers {
				ip, _ := node.FetchNodeExternalIP()
				cmd := "curl --header host:foo1.bar.com -m 5 -s -f http://" + ip + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-ingress"), "failed cmd: "+cmd)
			}
		})

		It("Verifies Daemonset", func() {
			_, err := tc.DeployWorkload("daemonset.yaml")
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")

			nodes, _ := e2e.ParseNodes(tc.KubeconfigFile, false)

			Eventually(func(g Gomega) {
				count, err := e2e.GetDaemonsetReady("test-daemonset", tc.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(nodes).To(HaveLen(count), "Daemonset pod count does not match node count")
			}, "240s", "10s").Should(Succeed())
		})

		It("Verifies dns access", func() {
			_, err := tc.DeployWorkload("dnsutils.yaml")
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")

			Eventually(func() (string, error) {
				cmd := "kubectl get pods dnsutils --kubeconfig=" + tc.KubeconfigFile
				return e2e.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("dnsutils"))

			cmd := "kubectl --kubeconfig=" + tc.KubeconfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("kubernetes.default.svc.cluster.local"))
		})

		It("Verifies Local Path Provisioner storage ", func() {
			_, err := tc.DeployWorkload("local-path-provisioner.yaml")
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed")
			Eventually(func(g Gomega) {
				cmd := "kubectl get pvc local-path-pvc --kubeconfig=" + tc.KubeconfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				fmt.Println(res)
				g.Expect(res).Should(ContainSubstring("local-path-pvc"))
				g.Expect(res).Should(ContainSubstring("Bound"))
			}, "240s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + tc.KubeconfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				fmt.Println(res)

				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			cmd := "kubectl --kubeconfig=" + tc.KubeconfigFile + " exec volume-test -- sh -c 'echo local-path-test > /data/test'"
			res, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)
			fmt.Println("Data stored in pvc: local-path-test")

			cmd = "kubectl delete pod volume-test --kubeconfig=" + tc.KubeconfigFile
			res, err = e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)

			_, err = tc.DeployWorkload("local-path-provisioner.yaml")
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed")

			Eventually(func() (string, error) {
				cmd := "kubectl get pods -o=name -l app=local-path-provisioner --field-selector=status.phase=Running -n kube-system --kubeconfig=" + tc.KubeconfigFile
				return e2e.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("local-path-provisioner"))

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + tc.KubeconfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				fmt.Println(res)
				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			// Check data after re-creation
			Eventually(func() (string, error) {
				cmd := "kubectl exec volume-test --kubeconfig=" + tc.KubeconfigFile + " -- cat /data/test"
				return e2e.RunCommand(cmd)
			}, "180s", "2s").Should(ContainSubstring("local-path-test"), "Failed to retrieve data from pvc")
		})

		It("Upgrades with no issues", func() {
			var err error
			Expect(e2e.UpgradeCluster(append(tc.Servers, tc.Agents...), *local)).To(Succeed())
			Expect(e2e.RestartCluster(append(tc.Servers, tc.Agents...))).To(Succeed())
			fmt.Println("CLUSTER UPGRADED")
			tc.KubeconfigFile, err = e2e.GenKubeconfigFile(tc.Servers[0].String())
			Expect(err).NotTo(HaveOccurred())
		})

		It("After upgrade Checks Node and Pod Status", func() {
			By("Fetching Nodes status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())
			e2e.ParseNodes(tc.KubeconfigFile, true)

			By("Fetching Pod status")
			tests.AllPodsUp(tc.KubeconfigFile)
			e2e.DumpPods(tc.KubeconfigFile)
		})

		It("After upgrade verifies ClusterIP Service", func() {
			Eventually(func() (string, error) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				return e2e.RunCommand(cmd)
			}, "420s", "5s").Should(ContainSubstring("test-clusterip"))

			clusterip, _ := e2e.FetchClusterIP(tc.KubeconfigFile, "nginx-clusterip-svc", false)
			cmd := "curl -m 5 -s -f http://" + clusterip + "/name.html"
			fmt.Println(cmd)
			for _, node := range tc.Servers {
				Eventually(func() (string, error) {
					return node.RunCmdOnNode(cmd)
				}, "120s", "10s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)
			}
		})

		It("After upgrade verifies NodePort Service", func() {

			for _, node := range tc.Servers {
				nodeExternalIP, _ := node.FetchNodeExternalIP()
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() (string, error) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")

				cmd = "curl -m 5 -s -f http://" + nodeExternalIP + ":" + nodeport + "/name.html"
				fmt.Println(cmd)
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"))
			}
		})

		It("After upgrade verifies LoadBalancer Service", func() {
			for _, node := range tc.Servers {
				ip, _ := node.FetchNodeExternalIP()
				cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].port}\""
				port, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() (string, error) {
					cmd := "curl -m 5 -s -f http://" + ip + ":" + port + "/name.html"
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"))

				Eventually(func() (string, error) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"))
			}
		})

		It("After upgrade verifies Ingress", func() {
			for _, node := range tc.Servers {
				ip, _ := node.FetchNodeExternalIP()
				cmd := "curl --header host:foo1.bar.com -m 5 -s -f http://" + ip + "/name.html"
				fmt.Println(cmd)

				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "420s", "5s").Should(ContainSubstring("test-ingress"))
			}
		})

		It("After upgrade verifies Daemonset", func() {
			nodes, _ := e2e.ParseNodes(tc.KubeconfigFile, false)

			Eventually(func(g Gomega) {
				count, err := e2e.GetDaemonsetReady("test-daemonset", tc.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(nodes).To(HaveLen(count), "Daemonset pod count does not match node count")
			}, "240s", "10s").Should(Succeed())
		})
		It("After upgrade verifies dns access", func() {
			Eventually(func() (string, error) {
				cmd := "kubectl --kubeconfig=" + tc.KubeconfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"
				return e2e.RunCommand(cmd)
			}, "180s", "2s").Should((ContainSubstring("kubernetes.default.svc.cluster.local")))
		})

		It("After upgrade verify Local Path Provisioner storage ", func() {
			Eventually(func() (string, error) {
				cmd := "kubectl exec volume-test --kubeconfig=" + tc.KubeconfigFile + " -- cat /data/test"
				return e2e.RunCommand(cmd)
			}, "180s", "2s").Should(ContainSubstring("local-path-test"))
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
