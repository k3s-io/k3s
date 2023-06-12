package validatecluster

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS:
// generic/ubuntu2004, generic/centos7, generic/rocky8,
// opensuse/Leap-15.3.x86_64
var nodeOS = flag.String("nodeOS", "generic/ubuntu2004", "VM operating system")
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

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Create", Ordered, func() {
	Context("Cluster Starts up and deploys basic components", func() {
		It("Starts up with no issues", func() {
			var err error
			if *local {
				serverNodeNames, agentNodeNames, err = e2e.CreateLocalCluster(*nodeOS, *serverCount, *agentCount)
			} else {
				serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
			}
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
			res, err := e2e.DeployWorkload("clusterip.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed: "+res)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				res, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should((ContainSubstring("test-clusterip")), "failed cmd: "+cmd+" result: "+res)
			}, "240s", "5s").Should(Succeed())

			clusterip, _ := e2e.FetchClusterIP(kubeConfigFile, "nginx-clusterip-svc", false)
			cmd := "curl -L --insecure http://" + clusterip + "/name.html"
			for _, nodeName := range serverNodeNames {
				Eventually(func(g Gomega) {
					res, err := e2e.RunCmdOnNode(cmd, nodeName)
					g.Expect(err).NotTo(HaveOccurred())
					Expect(res).Should(ContainSubstring("test-clusterip"))
				}, "120s", "10s").Should(Succeed())
			}
		})

		It("Verifies NodePort Service", func() {
			_, err := e2e.DeployWorkload("nodeport.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")

			for _, nodeName := range serverNodeNames {
				nodeExternalIP, _ := e2e.FetchNodeExternalIP(nodeName)
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func(g Gomega) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
					res, err := e2e.RunCommand(cmd)
					Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")
				}, "240s", "5s").Should(Succeed())

				cmd = "curl -L --insecure http://" + nodeExternalIP + ":" + nodeport + "/name.html"

				Eventually(func(g Gomega) {
					res, err := e2e.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd+" result: "+res)
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "240s", "5s").Should(Succeed())
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

				Eventually(func(g Gomega) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
					res, err := e2e.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd+" result: "+res)
					g.Expect(res).Should(ContainSubstring("test-loadbalancer"))
				}, "240s", "5s").Should(Succeed())

				Eventually(func(g Gomega) {
					cmd = "curl -L --insecure http://" + ip + ":" + port + "/name.html"
					res, err := e2e.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd+" result: "+res)
					g.Expect(res).Should(ContainSubstring("test-loadbalancer"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Ingress", func() {
			_, err := e2e.DeployWorkload("ingress.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")

			for _, nodeName := range serverNodeNames {
				ip, _ := e2e.FetchNodeExternalIP(nodeName)
				cmd := "curl  --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				fmt.Println(cmd)

				Eventually(func(g Gomega) {
					res, err := e2e.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd+" result: "+res)
					g.Expect(res).Should(ContainSubstring("test-ingress"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Daemonset", func() {
			_, err := e2e.DeployWorkload("daemonset.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")

			nodes, _ := e2e.ParseNodes(kubeConfigFile, false)

			Eventually(func(g Gomega) {
				pods, _ := e2e.ParsePods(kubeConfigFile, false)
				count := e2e.CountOfStringInSlice("test-daemonset", pods)
				fmt.Println("POD COUNT")
				fmt.Println(count)
				fmt.Println("NODE COUNT")
				fmt.Println(len(nodes))
				g.Expect(len(nodes)).Should((Equal(count)), "Daemonset pod count does not match node count")
			}, "420s", "10s").Should(Succeed())
		})

		It("Verifies dns access", func() {
			_, err := e2e.DeployWorkload("dnsutils.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods dnsutils --kubeconfig=" + kubeConfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd+" result: "+res)
				g.Expect(res).Should(ContainSubstring("dnsutils"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"

				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd+" result: "+res)
				g.Expect(res).Should(ContainSubstring("kubernetes.default.svc.cluster.local"))
			}, "420s", "2s").Should(Succeed())
		})

		It("Verifies Local Path Provisioner storage ", func() {
			res, err := e2e.DeployWorkload("local-path-provisioner.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed: "+res)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pvc local-path-pvc --kubeconfig=" + kubeConfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd+" result: "+res)
				g.Expect(res).Should(ContainSubstring("local-path-pvc"))
				g.Expect(res).Should(ContainSubstring("Bound"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + kubeConfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd+" result: "+res)
				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec volume-test -- sh -c 'echo local-path-test > /data/test'"
			_, err = e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = "kubectl delete pod volume-test --kubeconfig=" + kubeConfigFile
			res, err = e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd+" result: "+res)

			_, err = e2e.DeployWorkload("local-path-provisioner.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l app=local-path-provisioner --field-selector=status.phase=Running -n kube-system --kubeconfig=" + kubeConfigFile
				res, _ := e2e.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("local-path-provisioner"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + kubeConfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd+" result: "+res)

				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl exec volume-test --kubeconfig=" + kubeConfigFile + " -- cat /data/test"
				res, err = e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd+" result: "+res)
				fmt.Println("Data after re-creation", res)
				g.Expect(res).Should(ContainSubstring("local-path-test"))
			}, "180s", "2s").Should(Succeed())
		})
	})

	Context("Validate restart", func() {
		It("Restarts normally", func() {
			errRestart := e2e.RestartCluster(append(serverNodeNames, agentNodeNames...))
			Expect(errRestart).NotTo(HaveOccurred(), "Restart Nodes not happened correctly")

			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
				pods, _ := e2e.ParsePods(kubeConfigFile, false)
				count := e2e.CountOfStringInSlice("test-daemonset", pods)
				g.Expect(len(nodes)).Should((Equal(count)), "Daemonset pod count does not match node count")
				podsRunningAr := 0
				for _, pod := range pods {
					if strings.Contains(pod.Name, "test-daemonset") && pod.Status == "Running" && pod.Ready == "1/1" {
						podsRunningAr++
					}
				}
				g.Expect(len(nodes)).Should((Equal(podsRunningAr)), "Daemonset pods are not running after the restart")
			}, "620s", "5s").Should(Succeed())
		})
	})

	Context("Valdiate Certificate Rotation", func() {
		It("Stops K3s and rotates certificates", func() {
			errStop := e2e.StopCluster(serverNodeNames)
			Expect(errStop).NotTo(HaveOccurred(), "Cluster could not be stopped successfully")

			for _, nodeName := range serverNodeNames {
				cmd := "k3s certificate rotate"
				if _, err := e2e.RunCmdOnNode(cmd, nodeName); err != nil {
					Expect(err).NotTo(HaveOccurred(), "Certificate could not be rotated successfully")
				}
			}
		})

		It("Start normally", func() {
			// Since we stopped all the server, we have to start 2 at once to get it back up
			// If we only start one at a time, the first will hang waiting for the second to be up
			_, err := e2e.RunCmdOnNode("systemctl --no-block start k3s", serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			err = e2e.StartCluster(serverNodeNames[1:])
			Expect(err).NotTo(HaveOccurred(), "Cluster could not be started successfully")

			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
				fmt.Println("help")
			}, "620s", "5s").Should(Succeed())

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
		})
		It("Validates certificates", func() {
			const grepCert = "ls -lt /var/lib/rancher/k3s/server/ | grep tls"
			var expectResult = []string{
				"client-ca.crt", "client-ca.key", "client-ca.nochain.crt",
				"client-supervisor.crt", "client-supervisor.key",
				"dynamic-cert.json", "peer-ca.crt",
				"peer-ca.key", "server-ca.crt",
				"server-ca.key", "request-header-ca.crt",
				"request-header-ca.key", "server-ca.crt",
				"server-ca.key", "server-ca.nochain.crt",
				"service.current.key", "service.key",
				"apiserver-loopback-client__.crt", "apiserver-loopback-client__.key",
				"",
			}

			var finalResult string
			var finalErr error
			for _, nodeName := range serverNodeNames {
				grCert, errGrep := e2e.RunCmdOnNode(grepCert, nodeName)
				Expect(errGrep).NotTo(HaveOccurred(), "Certificate could not be created successfully")
				re := regexp.MustCompile("tls-[0-9]+")
				tls := re.FindAllString(grCert, -1)[0]
				final := fmt.Sprintf("diff -sr /var/lib/rancher/k3s/server/tls/ /var/lib/rancher/k3s/server/%s/"+
					"| grep -i identical | cut -f4 -d ' ' | xargs basename -a \n", tls)
				finalResult, finalErr = e2e.RunCmdOnNode(final, nodeName)
				Expect(finalErr).NotTo(HaveOccurred(), "Final Certification does not created successfully")
			}
			errRestartAgent := e2e.RestartCluster(agentNodeNames)
			Expect(errRestartAgent).NotTo(HaveOccurred(), "Agent could not be restart successfully")

			finalCert := strings.Replace(finalResult, "\n", ",", -1)
			finalCertArray := strings.Split(finalCert, ",")
			Expect((finalCertArray)).Should((Equal(expectResult)), "Final certification does not match the expected results")

		})

	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if os.Getenv("E2E_GOCOVER") != "" {
		Expect(e2e.GetCoverageReport(append(serverNodeNames, agentNodeNames...))).To(Succeed())
	}
	if failed && !*ci {
		fmt.Println("FAILED!")
	} else {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
