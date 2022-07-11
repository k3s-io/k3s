package terraform

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func Test_TFClusterCreateValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()

	RunSpecs(t, "Create Cluster Test Suite")
}

var _ = Describe("Test:", func() {
	Context("Build Cluster:", func() {
		It("Starts up with no issues", func() {
			status, err := BuildCluster(*nodeOs, *awsAmi, *clusterType, *externalDb, *resourceName, &testing.T{}, false, *arch)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal("cluster created"))
			defer GinkgoRecover()
			if strings.Contains(*clusterType, "etcd") {
				fmt.Println("\nCLUSTER CONFIG:\nOS", *nodeOs, "\nBACKEND", *clusterType)
			} else {
				fmt.Println("\nCLUSTER CONFIG:\nOS", *nodeOs, "\nBACKEND", *externalDb)
			}
			fmt.Printf("\nIPs:\n")
			fmt.Println("Server Node IPS:", masterIPs)
			fmt.Println("Agent Node IPS:", workerIPs)
			fmt.Println(kubeConfigFile)
			Expect(kubeConfigFile).Should(ContainSubstring(*resourceName))
		})

		It("Checks Node and Pod Status", func() {
			defer func() {
				_, err := ParseNodes(kubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving nodes: ", err)
				}
				_, err = ParsePods(kubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving pods: ", err)
				}
			}()

			fmt.Printf("\nFetching node status\n")
			expectedNodeCount := *serverNodes + *workerNodes + 1
			Eventually(func(g Gomega) {
				nodes, err := ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).To(Equal(expectedNodeCount), "Number of nodes should match the spec")
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"), "Nodes should all be in Ready state")
				}
			}, "420s", "5s").Should(Succeed())

			fmt.Printf("\nFetching pod status\n")
			Eventually(func(g Gomega) {
				pods, err := ParsePods(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
					} else {
						g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
						g.Expect(pod.Restarts).Should(Equal("0"), pod.Name)
					}
				}
			}, "600s", "5s").Should(Succeed())
		})

		It("Verifies ClusterIP Service", func() {
			_, err := DeployWorkload("clusterip.yaml", kubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				res, err := RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should((ContainSubstring("test-clusterip")))
			}, "420s", "5s").Should(Succeed())

			clusterip, _ := FetchClusterIP(kubeConfigFile, "nginx-clusterip-svc")
			cmd := "curl -sL --insecure http://" + clusterip + "/name.html"
			nodeExternalIP := FetchNodeExternalIP(kubeConfigFile)
			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					res, err := RunCmdOnNode(cmd, ip, *sshuser, *access_key)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-clusterip"))
				}, "420s", "10s").Should(Succeed())
			}
		})

		It("Verifies NodePort Service", func() {
			_, err := DeployWorkload("nodeport.yaml", kubeConfigFile, *arch)
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

				cmd = "curl -sL --insecure http://" + ip + ":" + nodeport + "/name.html"
				Eventually(func(g Gomega) {
					res, err := RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies LoadBalancer Service", func() {
			_, err := DeployWorkload("loadbalancer.yaml", kubeConfigFile, *arch)
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
					cmd = "curl -sL --insecure http://" + ip + ":" + port + "/name.html"
					res, err := RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-loadbalancer"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Ingress", func() {
			_, err := DeployWorkload("ingress.yaml", kubeConfigFile, *arch)
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
				cmd := "curl -s --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				Eventually(func(g Gomega) {
					res, err := RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-ingress"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Daemonset", func() {
			_, err := DeployWorkload("daemonset.yaml", kubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")

			nodes, _ := ParseNodes(kubeConfigFile, false)
			pods, _ := ParsePods(kubeConfigFile, false)

			Eventually(func(g Gomega) {
				count := CountOfStringInSlice("test-daemonset", pods)
				g.Expect(len(nodes)).Should((Equal(count)), "Daemonset pod count does not match node count")
			}, "420s", "10s").Should(Succeed())
		})

		It("Verifies Local Path Provisioner storage ", func() {
			_, err := DeployWorkload("local-path-provisioner.yaml", kubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pvc local-path-pvc --kubeconfig=" + kubeConfigFile
				res, err := RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("local-path-pvc"))
				g.Expect(res).Should(ContainSubstring("Bound"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + kubeConfigFile
				res, err := RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec volume-test -- sh -c 'echo local-path-test > /data/test'"
			_, err = RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = "kubectl delete pod volume-test --kubeconfig=" + kubeConfigFile
			_, err = RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			_, err = DeployWorkload("local-path-provisioner.yaml", kubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l app=local-path-provisioner --field-selector=status.phase=Running -n kube-system --kubeconfig=" + kubeConfigFile
				res, err := RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("pod/local-path-provisioner"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + kubeConfigFile
				res, err := RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd = "kubectl exec volume-test --kubeconfig=" + kubeConfigFile + " -- cat /data/test"
				res, err := RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("local-path-test"))
			}, "180s", "2s").Should(Succeed())
		})

		It("Verifies dns access", func() {
			_, err := DeployWorkload("dnsutils.yaml", kubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods dnsutils --kubeconfig=" + kubeConfigFile
				res, _ := RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("dnsutils"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec -t dnsutils -- nslookup kubernetes.default"
				res, _ := RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("kubernetes.default.svc.cluster.local"))
			}, "420s", "2s").Should(Succeed())
		})
	})
})

var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = BeforeEach(func() {
	failed = failed || CurrentSpecReport().Failed()
	if *destroy {
		fmt.Printf("\nCluster is being Deleted\n")
		Skip("Cluster is being Deleted")
	}

})
var _ = AfterSuite(func() {
	if failed {
		fmt.Println("FAILED!")
	} else if *destroy {
		status, err := BuildCluster(*nodeOs, *awsAmi, *clusterType, *externalDb, *resourceName, &testing.T{}, *destroy, *arch)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("cluster destroyed"))
	} else {
		fmt.Println("PASSED!")
	}
})
