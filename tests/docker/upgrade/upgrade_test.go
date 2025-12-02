package upgrade

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Using these two flags, we upgrade from the latest release of <branch> to
// the current commit build of K3s defined by <k3sImage>
var k3sImage = flag.String("k3sImage", "", "The current commit build of K3s")
var channel = flag.String("channel", "latest", "The release channel to test")
var ci = flag.Bool("ci", false, "running on CI, forced cleanup")
var tc *docker.TestConfig

var numServers = 3
var numAgents = 1

func Test_DockerUpgrade(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Upgrade Docker Test Suite")
}

var _ = Describe("Upgrade Tests", Ordered, func() {

	Context("Setup Cluster with Lastest Release", func() {
		var latestVersion string
		It("should determine latest branch version", func() {
			url := fmt.Sprintf("https://update.k3s.io/v1-release/channels/%s", *channel)
			resp, err := http.Head(url)
			// Cover the case where the branch does not exist yet,
			// such as a new unreleased minor version
			if err != nil || resp.StatusCode != http.StatusOK {
				*channel = "latest"
			}

			latestVersion, err = docker.GetVersionFromChannel(*channel)
			Expect(err).NotTo(HaveOccurred())
			Expect(latestVersion).To(ContainSubstring("v1."))
			fmt.Println("Using latest version: ", latestVersion)
		})
		It("should setup environment", func() {
			var err error
			tc, err = docker.NewTestConfig("rancher/k3s:" + latestVersion)
			testID := filepath.Base(tc.TestDir)
			Expect(err).NotTo(HaveOccurred())
			for i := 0; i < numServers; i++ {
				m1 := fmt.Sprintf("--mount type=volume,src=server-%d-%s-rancher,dst=/var/lib/rancher/k3s", i, testID)
				m2 := fmt.Sprintf("--mount type=volume,src=server-%d-%s-log,dst=/var/log", i, testID)
				m3 := fmt.Sprintf("--mount type=volume,src=server-%d-%s-etc,dst=/etc/rancher", i, testID)
				Expect(os.Setenv(fmt.Sprintf("SERVER_%d_DOCKER_ARGS", i), fmt.Sprintf("%s %s %s", m1, m2, m3))).To(Succeed())
			}
			for i := 0; i < numAgents; i++ {
				m1 := fmt.Sprintf("--mount type=volume,src=agent-%d-%s-rancher,dst=/var/lib/rancher/k3s", i, testID)
				m2 := fmt.Sprintf("--mount type=volume,src=agent-%d-%s-log,dst=/var/log", i, testID)
				m3 := fmt.Sprintf("--mount type=volume,src=agent-%d-%s-etc,dst=/etc/rancher", i, testID)
				Expect(os.Setenv(fmt.Sprintf("AGENT_%d_DOCKER_ARGS", i), fmt.Sprintf("%s %s %s", m1, m2, m3))).To(Succeed())
			}
		})
		It("should provision servers and agents", func() {
			Expect(tc.ProvisionServers(numServers)).To(Succeed())
			Expect(tc.ProvisionAgents(numAgents)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(tc.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
		})
		It("should confirm latest version", func() {
			for _, server := range tc.Servers {
				out, err := server.RunCmdOnNode("k3s --version")
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(ContainSubstring(strings.Replace(latestVersion, "-", "+", 1)))
			}
		})
	})
	Context("Validates resource functionality", func() {
		It("should deploy a test pod", func() {
			_, err := tc.DeployWorkload("volume-test.yaml")
			Expect(err).NotTo(HaveOccurred(), "failed to apply volume test manifest")

			Eventually(func() (bool, error) {
				return tests.PodReady("volume-test", "kube-system", tc.KubeconfigFile)
			}, "20s", "5s").Should(BeTrue())
		})
		It("Verifies ClusterIP Service", func() {
			_, err := tc.DeployWorkload("clusterip.yaml")

			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")

			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
			Eventually(func() (string, error) {
				return tests.RunCommand(cmd)
			}, "240s", "5s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)

			cmd = "kubectl get svc nginx-clusterip-svc -o jsonpath='{.spec.clusterIP}'"
			clusterip, _ := tests.RunCommand(cmd)
			cmd = "wget -T 5 -O - -q http://" + clusterip + "/name.html"
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
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, err := tests.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)

				cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				Eventually(func() (string, error) {
					return tests.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")

				cmd = "curl -m 5 -s -f http://" + node.IP + ":" + nodeport + "/name.html"
				fmt.Println(cmd)
				Eventually(func() (string, error) {
					return tests.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "failed cmd: "+cmd)
			}
		})

		It("Verifies LoadBalancer Service", func() {
			_, err := tc.DeployWorkload("loadbalancer-allTraffic.yaml")
			Expect(err).NotTo(HaveOccurred(), "Loadbalancer manifest not deployed")
			for _, node := range tc.Servers {
				cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].port}\""
				port, err := tests.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				Eventually(func() (string, error) {
					return tests.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"))

				cmd = "curl -m 5 -s -f http://" + node.IP + ":" + port + "/ip"
				Eventually(func() (string, error) {
					return tests.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("10.42"), "failed cmd: "+cmd)
			}
		})

		It("Verifies Ingress", func() {
			_, err := tc.DeployWorkload("ingress.yaml")
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")

			for _, node := range tc.Servers {
				cmd := "curl --header host:foo1.bar.com -m 5 -s -f http://" + node.IP + "/name.html"
				Eventually(func() (string, error) {
					return tests.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-ingress"), "failed cmd: "+cmd)
			}
		})

		It("Verifies Daemonset", func() {
			_, err := tc.DeployWorkload("daemonset.yaml")
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")

			nodes, _ := tests.ParseNodes(tc.KubeconfigFile)
			Eventually(func(g Gomega) {
				count, err := tests.GetDaemonsetReady("test-daemonset", tc.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(nodes).To(HaveLen(count), "Daemonset pod count does not match node count")
			}, "240s", "10s").Should(Succeed())
		})

		It("Verifies dns access", func() {
			_, err := tc.DeployWorkload("dnsutils.yaml")
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")

			Eventually(func() (string, error) {
				cmd := "kubectl get pods dnsutils --kubeconfig=" + tc.KubeconfigFile
				return tests.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("dnsutils"))

			cmd := "kubectl --kubeconfig=" + tc.KubeconfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"
			Eventually(func() (string, error) {
				return tests.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("kubernetes.default.svc.cluster.local"))
		})
	})
	Context("Upgrade to Current Commit Build", func() {
		It("should upgrade to current commit build", func() {
			By("Remove old servers and agents")
			for _, server := range tc.Servers {
				cmd := fmt.Sprintf("docker stop %s", server.Name)
				Expect(tests.RunCommand(cmd)).Error().NotTo(HaveOccurred())
				cmd = fmt.Sprintf("docker rm %s", server.Name)
				Expect(tests.RunCommand(cmd)).Error().NotTo(HaveOccurred())
				fmt.Printf("Stopped %s\n", server.Name)
			}
			tc.Servers = nil

			for _, agent := range tc.Agents {
				cmd := fmt.Sprintf("docker stop %s", agent.Name)
				Expect(tests.RunCommand(cmd)).Error().NotTo(HaveOccurred())
				cmd = fmt.Sprintf("docker rm %s", agent.Name)
				Expect(tests.RunCommand(cmd)).Error().NotTo(HaveOccurred())
			}
			tc.Agents = nil

			tc.K3sImage = *k3sImage
			Expect(tc.ProvisionServers(numServers)).To(Succeed())
			Expect(tc.ProvisionAgents(numAgents)).To(Succeed())

			Eventually(func() error {
				return tests.CheckDefaultDeployments(tc.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
		})
		It("should confirm commit version", func() {
			for _, server := range tc.Servers {
				Eventually(func(g Gomega) {
					g.Expect(docker.VerifyValidVersion(server, "kubectl")).To(Succeed())
					g.Expect(docker.VerifyValidVersion(server, "ctr")).To(Succeed())
					g.Expect(docker.VerifyValidVersion(server, "crictl")).To(Succeed())
				}).Should(Succeed())

				out, err := server.RunCmdOnNode("k3s --version")
				Expect(err).NotTo(HaveOccurred())
				cVersion := strings.Split(*k3sImage, ":")[1]
				cVersion = strings.Replace(cVersion, "-amd64", "", 1)
				cVersion = strings.Replace(cVersion, "-arm64", "", 1)
				cVersion = strings.Replace(cVersion, "-arm", "", 1)
				cVersion = strings.Replace(cVersion, "-k3s", "+k3s", 1)
				Expect(out).To(ContainSubstring(cVersion))
			}
		})
	})
	Context("Validates resource functionality post-upgrade", func() {
		It("should confirm test pod is still Running", func() {
			Eventually(func() (bool, error) {
				return tests.PodReady("volume-test", "kube-system", tc.KubeconfigFile)
			}, "20s", "5s").Should(BeTrue())
		})
		It("Verifies ClusterIP Service", func() {
			Eventually(func() (string, error) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				return tests.RunCommand(cmd)
			}, "420s", "5s").Should(ContainSubstring("test-clusterip"))

			cmd := "kubectl get svc nginx-clusterip-svc -o jsonpath='{.spec.clusterIP}'"
			clusterip, _ := tests.RunCommand(cmd)
			cmd = "wget -T 5 -O - -q http://" + clusterip + "/name.html"
			fmt.Println(cmd)
			for _, node := range tc.Servers {
				Eventually(func() (string, error) {
					return node.RunCmdOnNode(cmd)
				}, "120s", "10s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)
			}
		})

		It("Verifies NodePort Service", func() {

			for _, node := range tc.Servers {
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, err := tests.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() (string, error) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
					return tests.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")

				cmd = "curl -m 5 -s -f http://" + node.IP + ":" + nodeport + "/name.html"
				fmt.Println(cmd)
				Eventually(func() (string, error) {
					return tests.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"))
			}
		})

		It("Verifies LoadBalancer Service", func() {
			for _, node := range tc.Servers {
				cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].port}\""
				port, err := tests.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() (string, error) {
					cmd := "curl -m 5 -s -f http://" + node.IP + ":" + port + "/ip"
					return tests.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("10.42"))

				Eventually(func() (string, error) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
					return tests.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"))
			}
		})

		It("Verifies Ingress", func() {
			for _, node := range tc.Servers {
				cmd := "curl --header host:foo1.bar.com -m 5 -s -f http://" + node.IP + "/name.html"
				fmt.Println(cmd)

				Eventually(func() (string, error) {
					return tests.RunCommand(cmd)
				}, "420s", "5s").Should(ContainSubstring("test-ingress"))
			}
		})

		It("Verifies Daemonset", func() {
			nodes, _ := tests.ParseNodes(tc.KubeconfigFile)
			Eventually(func(g Gomega) {
				count, err := tests.GetDaemonsetReady("test-daemonset", tc.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(nodes).To(HaveLen(count), "Daemonset pod count does not match node count")
			}, "240s", "10s").Should(Succeed())
		})
		It("Verifies dns access", func() {
			Eventually(func() (string, error) {
				cmd := "kubectl --kubeconfig=" + tc.KubeconfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"
				return tests.RunCommand(cmd)
			}, "180s", "2s").Should((ContainSubstring("kubernetes.default.svc.cluster.local")))
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		AddReportEntry("describe", docker.DescribeNodesAndPods(tc))
		AddReportEntry("docker-containers", docker.ListContainers())
		AddReportEntry("docker-logs", docker.TailDockerLogs(1000, append(tc.Servers, tc.Agents...)))
	}
	if tc != nil && (*ci || !failed) {
		Expect(tc.Cleanup()).To(Succeed())
	}
})
