package upgradecluster

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

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Upgrade", Ordered, func() {
	Context("Cluster :", func() {
		It("Starts up with no issues", func() {
			var err error
			serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			fmt.Println("CLUSTER CONFIG")
			fmt.Println("OS:", *nodeOS)
			fmt.Println("Server Nodes:", serverNodeNames)
			fmt.Println("Agent Nodes:", agentNodeNames)
			kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks Node and Pod Status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(kubeConfigFile, true)

			fmt.Printf("\nFetching Pods status\n")
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

		It("Verifies ClusterIP Service", func() {
			_, err := e2e.DeployWorkload("clusterip.yaml", kubeConfigFile, *hardened)

			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")

			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "240s", "5s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)

			clusterip, _ := e2e.FetchClusterIP(kubeConfigFile, "nginx-clusterip-svc", false)
			cmd = "curl -L --insecure http://" + clusterip + "/name.html"
			for _, nodeName := range serverNodeNames {
				Eventually(func() (string, error) {
					return e2e.RunCmdOnNode(cmd, nodeName)
				}, "120s", "10s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)
			}
		})

		It("Verifies NodePort Service", func() {
			_, err := e2e.DeployWorkload("nodeport.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")

			for _, nodeName := range serverNodeNames {
				nodeExternalIP, _ := e2e.FetchNodeExternalIP(nodeName)
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)

				cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")

				cmd = "curl -L --insecure http://" + nodeExternalIP + ":" + nodeport + "/name.html"
				fmt.Println(cmd)
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "failed cmd: "+cmd)
			}
		})

		It("Verifies LoadBalancer Service", func() {
			_, err := e2e.DeployWorkload("loadbalancer.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "Loadbalancer manifest not deployed")
			for _, nodeName := range serverNodeNames {
				ip, _ := e2e.FetchNodeExternalIP(nodeName)
				cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].port}\""
				port, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"))

				cmd = "curl -L --insecure http://" + ip + ":" + port + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"), "failed cmd: "+cmd)
			}
		})

		It("Verifies Ingress", func() {
			_, err := e2e.DeployWorkload("ingress.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")

			for _, nodeName := range serverNodeNames {
				ip, _ := e2e.FetchNodeExternalIP(nodeName)
				cmd := "curl  --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-ingress"), "failed cmd: "+cmd)
			}
		})

		It("Verifies Daemonset", func() {
			_, err := e2e.DeployWorkload("daemonset.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")

			nodes, _ := e2e.ParseNodes(kubeConfigFile, false) //nodes :=

			Eventually(func(g Gomega) {
				pods, _ := e2e.ParsePods(kubeConfigFile, false)
				count := e2e.CountOfStringInSlice("test-daemonset", pods)
				fmt.Println("POD COUNT")
				fmt.Println(count)
				fmt.Println("NODE COUNT")
				fmt.Println(len(nodes))
				g.Expect(len(nodes)).Should((Equal(count)), "Daemonset pod count does not match node count")
			}, "240s", "10s").Should(Succeed())
		})

		It("Verifies dns access", func() {
			_, err := e2e.DeployWorkload("dnsutils.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")

			Eventually(func() (string, error) {
				cmd := "kubectl get pods dnsutils --kubeconfig=" + kubeConfigFile
				return e2e.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("dnsutils"))

			cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("kubernetes.default.svc.cluster.local"))
		})

		It("Verifies Local Path Provisioner storage ", func() {
			_, err := e2e.DeployWorkload("local-path-provisioner.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed")
			Eventually(func(g Gomega) {
				cmd := "kubectl get pvc local-path-pvc --kubeconfig=" + kubeConfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				fmt.Println(res)
				g.Expect(res).Should(ContainSubstring("local-path-pvc"))
				g.Expect(res).Should(ContainSubstring("Bound"))
			}, "240s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + kubeConfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				fmt.Println(res)

				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec volume-test -- sh -c 'echo local-path-test > /data/test'"
			_, err = e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			fmt.Println("Data stored in pvc: local-path-test")

			cmd = "kubectl delete pod volume-test --kubeconfig=" + kubeConfigFile
			res, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			fmt.Println(res)

			_, err = e2e.DeployWorkload("local-path-provisioner.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed")

			Eventually(func() (string, error) {
				cmd := "kubectl get pods -o=name -l app=local-path-provisioner --field-selector=status.phase=Running -n kube-system --kubeconfig=" + kubeConfigFile
				return e2e.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("local-path-provisioner"))

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + kubeConfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				fmt.Println(res)
				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			// Check data after re-creation
			Eventually(func() (string, error) {
				cmd := "kubectl exec volume-test --kubeconfig=" + kubeConfigFile + " -- cat /data/test"
				return e2e.RunCommand(cmd)
			}, "180s", "2s").Should(ContainSubstring("local-path-test"))
		})

		It("Upgrades with no issues", func() {
			var err error
			Expect(e2e.UpgradeCluster(append(serverNodeNames, agentNodeNames...), *local)).To(Succeed())
			Expect(e2e.RestartCluster(append(serverNodeNames, agentNodeNames...))).To(Succeed())
			fmt.Println("CLUSTER UPGRADED")
			kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
		})

		It("After upgrade Checks Node and Pod Status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())
			e2e.ParseNodes(kubeConfigFile, true)

			fmt.Printf("\nFetching Pods status\n")
			Eventually(func(g Gomega) {
				pods, err := e2e.ParsePods(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"))
					} else {
						g.Expect(pod.Status).Should(Equal("Running"))
					}
				}
			}, "420s", "5s").Should(Succeed())
			e2e.ParsePods(kubeConfigFile, true)
		})

		It("After upgrade verifies ClusterIP Service", func() {
			Eventually(func() (string, error) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				return e2e.RunCommand(cmd)
			}, "420s", "5s").Should(ContainSubstring("test-clusterip"))

			clusterip, _ := e2e.FetchClusterIP(kubeConfigFile, "nginx-clusterip-svc", false)
			cmd := "curl -L --insecure http://" + clusterip + "/name.html"
			fmt.Println(cmd)
			for _, nodeName := range serverNodeNames {
				Eventually(func() (string, error) {
					return e2e.RunCmdOnNode(cmd, nodeName)
				}, "120s", "10s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)
			}
		})

		It("After upgrade verifies NodePort Service", func() {

			for _, nodeName := range serverNodeNames {
				nodeExternalIP, _ := e2e.FetchNodeExternalIP(nodeName)
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() (string, error) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")

				cmd = "curl -L --insecure http://" + nodeExternalIP + ":" + nodeport + "/name.html"
				fmt.Println(cmd)
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"))
			}
		})

		It("After upgrade verifies LoadBalancer Service", func() {
			for _, nodeName := range serverNodeNames {
				ip, _ := e2e.FetchNodeExternalIP(nodeName)
				cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].port}\""
				port, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() (string, error) {
					cmd := "curl -L --insecure http://" + ip + ":" + port + "/name.html"
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"))

				Eventually(func() (string, error) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"))
			}
		})

		It("After upgrade verifies Ingress", func() {
			for _, nodeName := range serverNodeNames {
				ip, _ := e2e.FetchNodeExternalIP(nodeName)
				cmd := "curl  --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				fmt.Println(cmd)

				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "420s", "5s").Should(ContainSubstring("test-ingress"))
			}
		})

		It("After upgrade verifies Daemonset", func() {
			nodes, _ := e2e.ParseNodes(kubeConfigFile, false) //nodes :=

			Eventually(func(g Gomega) {
				pods, _ := e2e.ParsePods(kubeConfigFile, false)
				count := e2e.CountOfStringInSlice("test-daemonset", pods)
				fmt.Println("POD COUNT")
				fmt.Println(count)
				fmt.Println("NODE COUNT")
				fmt.Println(len(nodes))
				g.Expect(len(nodes)).Should(Equal(count), "Daemonset pod count does not match node count")
			}, "420s", "1s").Should(Succeed())
		})
		It("After upgrade verifies dns access", func() {
			Eventually(func() (string, error) {
				cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"
				return e2e.RunCommand(cmd)
			}, "180s", "2s").Should((ContainSubstring("kubernetes.default.svc.cluster.local")))
		})

		It("After upgrade verify Local Path Provisioner storage ", func() {
			Eventually(func() (string, error) {
				cmd := "kubectl exec volume-test --kubeconfig=" + kubeConfigFile + " -- cat /data/test"
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
	if !failed {
		Expect(e2e.GetCoverageReport(append(serverNodeNames, agentNodeNames...))).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
