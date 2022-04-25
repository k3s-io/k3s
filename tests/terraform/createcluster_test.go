package e2e

import (
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func Test_E2EClusterCreateValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	RunSpecs(t, "Create Cluster Test Suite")
}

var _ = Describe("Test:", func() {
	Context("Build Cluster:", func() {
		It("Starts up with no issues", func() {
			kubeConfigFile, masterIPs, workerIPs, err = BuildCluster(*nodeOs, *clusterType, *externalDb, *resourceName, &testing.T{}, *destroy)
			Expect(err).NotTo(HaveOccurred())
			defer GinkgoRecover()
			if *destroy {
				fmt.Printf("\nCluster is being Deleted\n")
				return
			}
			fmt.Println("\nCLUSTER CONFIG:\nOS", *nodeOs, "BACKEND", *clusterType, *externalDb)
			fmt.Printf("\nIPs:\n")
			fmt.Println("Server Node IPS:", masterIPs)
			fmt.Println("Agent Node IPS:", workerIPs)
			fmt.Println(kubeConfigFile)
			Expect(kubeConfigFile).Should(ContainSubstring(*resourceName))
		})

		It("Checks Node and Pod Status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())
			_, _ = ParseNodes(kubeConfigFile, true)

			fmt.Printf("\nFetching Pods status\n")
			Eventually(func(g Gomega) {
				pods, err := ParsePods(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
					} else {
						g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
					}
				}
			}, "420s", "5s").Should(Succeed())
			_, _ = ParsePods(kubeConfigFile, true)
		})

		It("Verifies ClusterIP Service", func() {
			_, err := DeployWorkload("clusterip.yaml", kubeConfigFile, false)
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				res, err := RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should((ContainSubstring("test-clusterip")))
			}, "420s", "5s").Should(Succeed())

			clusterip, _ := FetchClusterIP(kubeConfigFile, "nginx-clusterip-svc")
			cmd := "curl -L --insecure http://" + clusterip + "/name.html"
			fmt.Println(cmd)
			nodeExternalIP := FetchNodeExternalIP(kubeConfigFile)
			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					res, err := RunCmdOnNode(cmd, ip, *sshuser, *sshkey)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-clusterip"))

				}, "420s", "10s").Should(Succeed())
			}
		})

		It("Verifies NodePort Service", func() {
			_, err := DeployWorkload("nodeport.yaml", kubeConfigFile, false)
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")
			nodeExternalIP := FetchNodeExternalIP(kubeConfigFile)
			cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport, err := RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
					res, err := RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "240s", "5s").Should(Succeed())

				cmd = "curl -L --insecure http://" + ip + ":" + nodeport + "/name.html"
				fmt.Println(cmd)
				Eventually(func(g Gomega) {
					res, err := RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					fmt.Println(res)
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies LoadBalancer Service", func() {
			_, err := DeployWorkload("loadbalancer.yaml", kubeConfigFile, false)
			Expect(err).NotTo(HaveOccurred(), "Loadbalancer manifest not deployed")
			nodeExternalIP := FetchNodeExternalIP(kubeConfigFile)
			cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].port}\""
			port, err := RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			for _, ip := range nodeExternalIP {

				Eventually(func(g Gomega) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
					res, err := RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-loadbalancer"))
				}, "240s", "5s").Should(Succeed())

				Eventually(func(g Gomega) {
					cmd = "curl -L --insecure http://" + ip + ":" + port + "/name.html"
					fmt.Println(cmd)
					res, err := RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					fmt.Println(res)
					g.Expect(res).Should(ContainSubstring("test-loadbalancer"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Ingress", func() {
			_, err := DeployWorkload("ingress.yaml", kubeConfigFile, false)
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-ingress --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				res, err := RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("test-ingress"))
			}, "240s", "5s").Should(Succeed())

			ingressIps, err := FetchIngressIP(kubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Ingress ip is not returned")

			for _, ip := range ingressIps {
				cmd := "curl  --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				fmt.Println(cmd)

				Eventually(func(g Gomega) {
					res, err := RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-ingress"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Daemonset", func() {
			_, err := DeployWorkload("daemonset.yaml", kubeConfigFile, false)
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")

			nodes, _ := ParseNodes(kubeConfigFile, false)
			pods, _ := ParsePods(kubeConfigFile, false)

			Eventually(func(g Gomega) {
				count := CountOfStringInSlice("test-daemonset", pods)
				fmt.Println("POD COUNT")
				fmt.Println(count)
				fmt.Println("NODE COUNT")
				fmt.Println(len(nodes))
				g.Expect(len(nodes)).Should((Equal(count)), "Daemonset pod count does not match node count")
			}, "420s", "10s").Should(Succeed())
		})

		It("Verifies Local Path Provisioner storage ", func() {
			_, err := DeployWorkload("local-path-provisioner.yaml", kubeConfigFile, false)
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pvc local-path-pvc --kubeconfig=" + kubeConfigFile
				res, err := RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				fmt.Println(res)
				g.Expect(res).Should(ContainSubstring("local-path-pvc"))
				g.Expect(res).Should(ContainSubstring("Bound"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + kubeConfigFile
				res, err := RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
				fmt.Println(res)
				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec volume-test -- sh -c 'echo local-path-test > /data/test'"
			_, err = RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			fmt.Println("Data stored in pvc: local-path-test")

			cmd = "kubectl delete pod volume-test --kubeconfig=" + kubeConfigFile
			res, err := RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			fmt.Println(res)

			_, err = DeployWorkload("local-path-provisioner.yaml", kubeConfigFile, false)
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l app=local-path-provisioner --field-selector=status.phase=Running -n kube-system --kubeconfig=" + kubeConfigFile
				res, _ := RunCommand(cmd)
				fmt.Println(res)
				g.Expect(res).Should(ContainSubstring("pod/local-path-provisioner"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + kubeConfigFile
				res, err := RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				fmt.Println(res)
				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd = "kubectl exec volume-test cat /data/test --kubeconfig=" + kubeConfigFile
				res, err = RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				fmt.Println("Data after re-creation", res)
				g.Expect(res).Should(ContainSubstring("local-path-test"))
			}, "180s", "2s").Should(Succeed())
		})

		It("Verifies dns access", func() {
			_, err := DeployWorkload("dnsutils.yaml", kubeConfigFile, false)
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods dnsutils --kubeconfig=" + kubeConfigFile
				res, _ := RunCommand(cmd)
				fmt.Println(res)
				g.Expect(res).Should(ContainSubstring("dnsutils"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec -t dnsutils -- nslookup kubernetes.default"
				res, _ := RunCommand(cmd)
				fmt.Println(res)
				g.Expect(res).Should(ContainSubstring("kubernetes.default.svc.cluster.local"))

			}, "420s", "2s").Should(Succeed())
		})

		It("Validate Rebooting nodes", func() {
			if *destroy {
				return
			}
			defer GinkgoRecover()
			nodeExternalIP := FetchNodeExternalIP(kubeConfigFile)
			for _, ip := range nodeExternalIP {
				fmt.Println("\nRebooting node: ", ip)
				cmd := "ssh -i " + *sshkey + " -o \"StrictHostKeyChecking no\" " + *sshuser + "@" + ip + " sudo reboot"
				_, _ = RunCommand(cmd)
				time.Sleep(3 * time.Minute)
				fmt.Println("\nNode and Pod Status after rebooting node: ", ip)

				Eventually(func(g Gomega) {
					nodes, err := ParseNodes(kubeConfigFile, false)
					g.Expect(err).NotTo(HaveOccurred())
					for _, node := range nodes {
						g.Expect(node.Status).Should(Equal("Ready"))
					}
				}, "420s", "5s").Should(Succeed())
				_, _ = ParseNodes(kubeConfigFile, true)

				Eventually(func(g Gomega) {
					pods, err := ParsePods(kubeConfigFile, false)
					g.Expect(err).NotTo(HaveOccurred())
					for _, pod := range pods {
						if strings.Contains(pod.Name, "helm-install") {
							g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
						} else {
							g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
						}
					}
				}, "420s", "5s").Should(Succeed())
				_, _ = ParsePods(kubeConfigFile, true)
			}
		})
	})
})

var failed = false
var _ = AfterEach(func() {
	failed = failed || CurrentGinkgoTestDescription().Failed
})

var _ = AfterSuite(func() {
	if failed {
		fmt.Println("FAILED!")
	} else {
		kubeConfigFile, masterIPs, workerIPs, err = BuildCluster(*nodeOs, *clusterType, *externalDb, *resourceName, &testing.T{}, true)
		if err != nil {
			fmt.Println("Error Destroying Cluster", err)
		}
	}
})
