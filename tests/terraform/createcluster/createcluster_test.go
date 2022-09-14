package createcluster

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	tf "github.com/k3s-io/k3s/tests/terraform"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var awsAmi = flag.String("aws_ami", "", "a valid ami string like ami-abcxyz123")
var nodeOs = flag.String("node_os", "ubuntu", "a string")
var externalDb = flag.String("external_db", "mysql", "a string")
var arch = flag.String("arch", "amd64", "a string")
var clusterType = flag.String("cluster_type", "etcd", "a string")
var resourceName = flag.String("resource_name", "etcd", "a string")
var sshuser = flag.String("sshuser", "ubuntu", "a string")
var sshkey = flag.String("sshkey", "", "a string")
var accessKey = flag.String("access_key", "", "local path to the private sshkey")
var serverNodes = flag.Int("no_of_server_nodes", 2, "count of server nodes")
var workerNodes = flag.Int("no_of_worker_nodes", 1, "count of worker nodes")

var tfVars = flag.String("tfvars", "/tests/terraform/modules/k3scluster/config/local.tfvars", "custom .tfvars file from base project path")
var destroy = flag.Bool("destroy", false, "a bool")

var failed = false
var terraformOptions map[string]interface{}

func Test_TFClusterCreateValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()

	RunSpecs(t, "Create Cluster Test Suite")
}

var _ = BeforeSuite(func() {
	terraformOptions = ClusterOptions(NodeOs(*nodeOs), AwsAmi(*awsAmi), ClusterType(*clusterType), ExternalDb(*externalDb), ResourceName(*resourceName), AccessKey(*accessKey), Sshuser(*sshuser), ServerNodes(*serverNodes), WorkerNodes(*workerNodes), Sshkey(*sshkey))
})

var _ = Describe("Test:", func() {
	Context("Build Cluster:", func() {
		It("Starts up with no issues", func() {
			status, err := BuildCluster(&testing.T{}, *tfVars, false, terraformOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal("cluster created"))
			defer GinkgoRecover()
			if strings.Contains(*clusterType, "etcd") {
				fmt.Println("\nCLUSTER CONFIG:\nOS", *nodeOs, "\nBACKEND", *clusterType)
			} else {
				fmt.Println("\nCLUSTER CONFIG:\nOS", *nodeOs, "\nBACKEND", *externalDb)
			}
			fmt.Printf("\nIPs:\n")
			fmt.Println("Server Node IPS:", MasterIPs)
			fmt.Println("Agent Node IPS:", WorkerIPs)
			fmt.Println(KubeConfigFile)
			Expect(KubeConfigFile).Should(ContainSubstring(*resourceName))
		})

		It("Checks Node and Pod Status", func() {
			defer func() {
				_, err := tf.ParseNodes(KubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving nodes: ", err)
				}
				_, err = tf.ParsePods(KubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving pods: ", err)
				}
			}()

			fmt.Printf("\nFetching node status\n")
			expectedNodeCount := *serverNodes + *workerNodes + 1
			Eventually(func(g Gomega) {
				nodes, err := tf.ParseNodes(KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).To(Equal(expectedNodeCount), "Number of nodes should match the spec")
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"), "Nodes should all be in Ready state")
				}
			}, "420s", "5s").Should(Succeed())

			fmt.Printf("\nFetching pod status\n")
			Eventually(func(g Gomega) {
				pods, err := tf.ParsePods(KubeConfigFile, false)
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
			_, err := tf.DeployWorkload("clusterip.yaml", KubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + KubeConfigFile
				res, err := tf.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should((ContainSubstring("test-clusterip")))
			}, "420s", "5s").Should(Succeed())

			clusterip, _ := tf.FetchClusterIP(KubeConfigFile, "nginx-clusterip-svc")
			cmd := "curl -sL --insecure http://" + clusterip + "/name.html"
			nodeExternalIP := tf.FetchNodeExternalIP(KubeConfigFile)
			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					res, err := tf.RunCmdOnNode(cmd, ip, *sshuser, *accessKey)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-clusterip"))
				}, "420s", "10s").Should(Succeed())
			}
		})

		It("Verifies NodePort Service", func() {
			_, err := tf.DeployWorkload("nodeport.yaml", KubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")
			nodeExternalIP := tf.FetchNodeExternalIP(KubeConfigFile)
			cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + KubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport, err := tf.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + KubeConfigFile
					res, err := tf.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "240s", "5s").Should(Succeed())

				cmd = "curl -sL --insecure http://" + ip + ":" + nodeport + "/name.html"
				Eventually(func(g Gomega) {
					res, err := tf.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies LoadBalancer Service", func() {
			_, err := tf.DeployWorkload("loadbalancer.yaml", KubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "Loadbalancer manifest not deployed")
			nodeExternalIP := tf.FetchNodeExternalIP(KubeConfigFile)
			cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + KubeConfigFile + " --output jsonpath=\"{.spec.ports[0].port}\""
			port, err := tf.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			for _, ip := range nodeExternalIP {

				Eventually(func(g Gomega) {
					cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + KubeConfigFile
					res, err := tf.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-loadbalancer"))
				}, "240s", "5s").Should(Succeed())

				Eventually(func(g Gomega) {
					cmd = "curl -sL --insecure http://" + ip + ":" + port + "/name.html"
					res, err := tf.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-loadbalancer"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Ingress", func() {
			_, err := tf.DeployWorkload("ingress.yaml", KubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-ingress --field-selector=status.phase=Running --kubeconfig=" + KubeConfigFile
				res, err := tf.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("test-ingress"))
			}, "240s", "5s").Should(Succeed())

			ingressIps, err := tf.FetchIngressIP(KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Ingress ip is not returned")

			for _, ip := range ingressIps {
				cmd := "curl -s --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				Eventually(func(g Gomega) {
					res, err := tf.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-ingress"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Daemonset", func() {
			_, err := tf.DeployWorkload("daemonset.yaml", KubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")

			nodes, _ := tf.ParseNodes(KubeConfigFile, false)
			pods, _ := tf.ParsePods(KubeConfigFile, false)

			Eventually(func(g Gomega) {
				count := tf.CountOfStringInSlice("test-daemonset", pods)
				g.Expect(len(nodes)).Should((Equal(count)), "Daemonset pod count does not match node count")
			}, "420s", "10s").Should(Succeed())
		})

		It("Verifies Local Path Provisioner storage ", func() {
			_, err := tf.DeployWorkload("local-path-provisioner.yaml", KubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pvc local-path-pvc --kubeconfig=" + KubeConfigFile
				res, err := tf.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("local-path-pvc"))
				g.Expect(res).Should(ContainSubstring("Bound"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + KubeConfigFile
				res, err := tf.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			cmd := "kubectl --kubeconfig=" + KubeConfigFile + " exec volume-test -- sh -c 'echo local-path-test > /data/test'"
			_, err = tf.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = "kubectl delete pod volume-test --kubeconfig=" + KubeConfigFile
			_, err = tf.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			_, err = tf.DeployWorkload("local-path-provisioner.yaml", KubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "local-path-provisioner manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l app=local-path-provisioner --field-selector=status.phase=Running -n kube-system --kubeconfig=" + KubeConfigFile
				res, err := tf.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("pod/local-path-provisioner"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl get pod volume-test --kubeconfig=" + KubeConfigFile
				res, err := tf.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("volume-test"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl exec volume-test --kubeconfig=" + KubeConfigFile + " -- cat /data/test"
				res, err := tf.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("local-path-test"))
			}, "180s", "2s").Should(Succeed())
		})

		It("Verifies dns access", func() {
			_, err := tf.DeployWorkload("dnsutils.yaml", KubeConfigFile, *arch)
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods dnsutils --kubeconfig=" + KubeConfigFile
				res, _ := tf.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("dnsutils"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl --kubeconfig=" + KubeConfigFile + " exec -t dnsutils -- nslookup kubernetes.default"
				res, _ := tf.RunCommand(cmd)
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
		Skip("Cluster is being Deleted")
	}
})

var _ = AfterSuite(func() {
	if failed {
		fmt.Println("FAILED!")
	} else {
		fmt.Println("PASSED!")
	}
	if *destroy {
		status, err := BuildCluster(&testing.T{}, *tfVars, *destroy, terraformOptions)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("cluster destroyed"))
	}
})
