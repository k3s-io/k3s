package validatecluster

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS:
// bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
// eurolinux-vagrant/rocky-8, eurolinux-vagrant/rocky-9,
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 2, "number of agent nodes")
var hardened = flag.Bool("hardened", false, "true or false")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_EXTERNAL_DB: mysql, postgres, etcd (default: etcd)
// E2E_RELEASE_VERSION=v1.23.1+k3s2 (default: latest commit from master)
// E2E_REGISTRY: true/false (default: false)

func Test_E2EClusterValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Create Cluster Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Create", Ordered, func() {
	Context("Cluster Starts up and deploys basic components", func() {
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

		It("Checks node and pod status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(tc.KubeconfigFile, true)

			fmt.Printf("\nFetching Pods status\n")
			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "620s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})

		It("Verifies ClusterIP Service", func() {
			res, err := tc.DeployWorkload("clusterip.yaml")
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed: "+res)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				res, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should((ContainSubstring("test-clusterip")), "failed cmd: %q result: %s", cmd, res)
			}, "240s", "5s").Should(Succeed())

			clusterip, _ := e2e.FetchClusterIP(tc.KubeconfigFile, "nginx-clusterip-svc", false)
			cmd := "curl -m 5 -s -f http://" + clusterip + "/name.html"
			for _, node := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := node.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					Expect(res).Should(ContainSubstring("test-clusterip"))
				}, "120s", "10s").Should(Succeed())
			}
		})

		It("Verifies NodePort Service", func() {
			_, err := tc.DeployWorkload("nodeport.yaml")
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")

			for _, node := range tc.Servers {
				nodeExternalIP, _ := node.FetchNodeExternalIP()
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func(g Gomega) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
					res, err := e2e.RunCommand(cmd)
					Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")
				}, "240s", "5s").Should(Succeed())

				cmd = "curl -m 5 -s -f http://" + nodeExternalIP + ":" + nodeport + "/name.html"

				Eventually(func(g Gomega) {
					res, err := e2e.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "240s", "5s").Should(Succeed())
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

				Eventually(func(g Gomega) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
					res, err := e2e.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)
					g.Expect(res).Should(ContainSubstring("test-loadbalancer"))
				}, "240s", "5s").Should(Succeed())

				Eventually(func(g Gomega) {
					cmd = "curl -m 5 -s -f http://" + ip + ":" + port + "/name.html"
					res, err := e2e.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)
					g.Expect(res).Should(ContainSubstring("test-loadbalancer"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Ingress", func() {
			_, err := tc.DeployWorkload("ingress.yaml")
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")

			for _, node := range tc.Servers {
				ip, _ := node.FetchNodeExternalIP()
				cmd := "curl --header host:foo1.bar.com -m 5 -s -f http://" + ip + "/name.html"
				fmt.Println(cmd)

				Eventually(func(g Gomega) {
					res, err := e2e.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)
					g.Expect(res).Should(ContainSubstring("test-ingress"))
				}, "240s", "5s").Should(Succeed())
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

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods dnsutils --kubeconfig=" + tc.KubeconfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)
				g.Expect(res).Should(ContainSubstring("dnsutils"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl --kubeconfig=" + tc.KubeconfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"

				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)
				g.Expect(res).Should(ContainSubstring("kubernetes.default.svc.cluster.local"))
			}, "420s", "2s").Should(Succeed())
		})

		It("Verifies Local Path Provisioner storage ", func() {
			res, err := tc.DeployWorkload("local-path-provisioner.yaml")
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed: "+res)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pvc local-path-pvc --kubeconfig=" + tc.KubeconfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)
				g.Expect(res).Should(ContainSubstring("local-path-pvc"))
				g.Expect(res).Should(ContainSubstring("Bound"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + tc.KubeconfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)
				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			cmd := "kubectl --kubeconfig=" + tc.KubeconfigFile + " exec volume-test -- sh -c 'echo local-path-test > /data/test'"
			res, err = e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)

			cmd = "kubectl delete pod volume-test --kubeconfig=" + tc.KubeconfigFile
			res, err = e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)

			_, err = tc.DeployWorkload("local-path-provisioner.yaml")
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l app=local-path-provisioner --field-selector=status.phase=Running -n kube-system --kubeconfig=" + tc.KubeconfigFile
				res, _ := e2e.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("local-path-provisioner"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + tc.KubeconfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)

				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl exec volume-test --kubeconfig=" + tc.KubeconfigFile + " -- cat /data/test"
				res, err = e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: %q result: %s", cmd, res)
				fmt.Println("Data after re-creation", res)
				g.Expect(res).Should(ContainSubstring("local-path-test"))
			}, "180s", "2s").Should(Succeed())
		})
	})

	Context("Validate restart", func() {
		It("Restarts normally", func() {
			errRestart := e2e.RestartCluster(append(tc.Servers, tc.Agents...))
			Expect(errRestart).NotTo(HaveOccurred(), "Restart Nodes not happened correctly")

			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
				count, err := e2e.GetDaemonsetReady("test-daemonset", tc.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).Should((Equal(count)), "Daemonset pods that are ready does not match node count")
			}, "620s", "5s").Should(Succeed())
		})
	})

	Context("Valdiate Certificate Rotation", func() {
		It("Stops K3s and rotates certificates", func() {
			errStop := e2e.StopCluster(tc.Servers)
			Expect(errStop).NotTo(HaveOccurred(), "Cluster could not be stopped successfully")

			for _, node := range tc.Servers {
				cmd := "k3s certificate rotate"
				_, err := node.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred(), "Certificate could not be rotated successfully on "+node.String())
			}
		})

		It("Start normally", func() {
			// Since we stopped all the server, we have to start 2 at once to get it back up
			// If we only start one at a time, the first will hang waiting for the second to be up
			_, err := tc.Servers[0].RunCmdOnNode("systemctl --no-block start k3s")
			Expect(err).NotTo(HaveOccurred())
			err = e2e.StartCluster(tc.Servers[1:])
			Expect(err).NotTo(HaveOccurred(), "Cluster could not be started successfully")

			Eventually(func(g Gomega) {
				for _, node := range tc.Servers {
					cmd := "test ! -e /var/lib/rancher/k3s/server/tls/dynamic-cert-regenerate"
					_, err := node.RunCmdOnNode(cmd)
					Expect(err).NotTo(HaveOccurred(), "Dynamic cert regenerate file not removed on "+node.String())
				}
			}, "620s", "5s").Should(Succeed())

			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "620s", "5s").Should(Succeed())
		})

		It("Validates certificates", func() {
			const grepCert = "ls -lt /var/lib/rancher/k3s/server/ | grep tls"
			// This is a list of files that should be IDENTICAL after certificates are rotated.
			// Everything else should be changed.
			var expectResult = []string{
				"client-ca.crt", "client-ca.key", "client-ca.nochain.crt",
				"peer-ca.crt", "peer-ca.key",
				"server-ca.crt", "server-ca.key",
				"request-header-ca.crt", "request-header-ca.key",
				"server-ca.crt", "server-ca.key", "server-ca.nochain.crt",
				"service.current.key", "service.key",
				"apiserver-loopback-client__.crt", "apiserver-loopback-client__.key",
				"",
			}

			for _, node := range tc.Servers {
				grCert, errGrep := node.RunCmdOnNode(grepCert)
				Expect(errGrep).NotTo(HaveOccurred(), "TLS dirs could not be listed on "+node.String())
				re := regexp.MustCompile("tls-[0-9]+")
				tls := re.FindAllString(grCert, -1)[0]
				diff := fmt.Sprintf("diff -sr /var/lib/rancher/k3s/server/tls/ /var/lib/rancher/k3s/server/%s/"+
					"| grep -i identical | cut -f4 -d ' ' | xargs basename -a \n", tls)
				result, err := node.RunCmdOnNode(diff)
				Expect(err).NotTo(HaveOccurred(), "Certificate diff not created successfully on "+node.String())

				certArray := strings.Split(result, "\n")
				Expect((certArray)).Should((Equal(expectResult)), "Certificate diff does not match the expected results on "+node.String())
			}

			errRestartAgent := e2e.RestartCluster(tc.Agents)
			Expect(errRestartAgent).NotTo(HaveOccurred(), "Agent could not be restart successfully")
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
