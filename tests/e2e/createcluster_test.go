package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	"strings"
	"testing"
	"time"
)

func Test_E2EClusterCreateValidation(t *testing.T) {
	junitReporter := reporters.NewJUnitReporter(fmt.Sprintf("/config/" + *resourceName + ".xml"))
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Test Suite", []Reporter{junitReporter})

}

var _ = Describe("Test:", func() {
	Context("Build Cluster:", func() {
		Context("Cluster Configuration: OS: "+*nodeOs+"  Cluster Type; "+*externalDb+" "+*clusterType, func() {

			kubeconfig, masterIPs, workerIPs = BuildCluster(*nodeOs, *clusterType, *externalDb, *resourceName, &testing.T{}, *destroy)
			defer GinkgoRecover()
			if *destroy {
				fmt.Printf("\nCluster is being Deleted\n")
				return
			}
			fmt.Println("\nCLUSTER CONFIG:\nOS", *nodeOs, "BACKEND", *clusterType, *externalDb)
			fmt.Printf("\nIPs:\n")
			fmt.Println("Master Node IPS:", masterIPs)
			fmt.Println("Worker Node IPS:", workerIPs)

			fmt.Printf("\nFetching node status\n")
			nodes := ParseNode(kubeconfig, true)
			for _, config := range nodes {
				Expect(config.Status).Should(Equal("Ready"), func() string { return config.Name })
			}

			fmt.Printf("\nFetching Pods status\n")
			pods := ParsePod(kubeconfig, true)
			for _, pod := range pods {
				if strings.Contains(pod.Name, "helm-install") {
					Expect(pod.Status).Should(Equal("Completed"), func() string { return pod.Name })
				} else {
					Expect(pod.Status).Should(Equal("Running"), func() string { return pod.Name })
				}
			}
		})
		Context("Validate Rebooting nodes", func() {
			if *destroy {
				return
			}
			defer GinkgoRecover()
			nodeExternalIP := FetchNodeExternalIP(kubeconfig)
			for _, ip := range nodeExternalIP {
				fmt.Println("\nRebooting node: ", ip)
				cmd := "ssh -i " + *sshkey + " -o \"StrictHostKeyChecking no\" " + *sshuser + "@" + ip + " sudo reboot"
				_, _ = RunCommand(cmd)
				time.Sleep(3 * time.Minute)
				fmt.Println("\nNode and Pod Status after rebooting node: ", ip)
				nodes := ParseNode(kubeconfig, true)
				for _, config := range nodes {
					Expect(config.Status).Should(Equal("Ready"), func() string { return config.Name })
				}

				pods := ParsePod(kubeconfig, true)
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						Expect(pod.Status).Should(Equal("Completed"), func() string { return pod.Name })
					} else {
						Expect(pod.Status).Should(Equal("Running"), func() string { return pod.Name })
					}
				}
			}
		})

		Context("Deploy workloads ", func() {
			if *destroy {
				return
			}
			defer GinkgoRecover()

			It("Validate Cluster IP", func() {
				if *destroy {
					return
				}
				DeployWorkloads(*arch, kubeconfig)
				fmt.Println("Validating ClusterIP")
				clusterip := FetchClusterIP(kubeconfig, "nginx-clusterip-svc")
				cmd := "curl -L --insecure http://" + clusterip + "/name.html"
				fmt.Println(cmd)
				//Fetch External IP to login to node and validate cluster ip
				nodeExternalIP := FetchNodeExternalIP(kubeconfig)

				for _, ip := range nodeExternalIP {
					res := RunCmdOnNode(cmd, ip, *sshuser, *sshkey)
					fmt.Println(res)
					Expect(res).Should(ContainSubstring("test-clusterip"), func() string { return res })
				}
			})

			It("Validate NodePort", func() {
				if *destroy {
					return
				}
				fmt.Println("Validating NodePort")
				nodeExternalIP := FetchNodeExternalIP(kubeconfig)
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + kubeconfig + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, _ := RunCommand(cmd)
				for _, nodeExternalIp := range nodeExternalIP {
					cmd := "curl -L --insecure http://" + nodeExternalIp + ":" + nodeport + "/name.html"
					fmt.Println(cmd)
					res, _ := RunCommand(cmd)
					fmt.Println(res)
					Expect(res).Should(ContainSubstring("test-nodeport"), func() string { return res })
				}
			})

			It("Validate LoadBalancer", func() {
				if *destroy {
					return
				}
				fmt.Println("Validating Service LoadBalancer")
				nodeExternalIP := FetchNodeExternalIP(kubeconfig)
				cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + kubeconfig + " --output jsonpath=\"{.spec.ports[0].port}\""
				port, _ := RunCommand(cmd)
				for _, ip := range nodeExternalIP {
					cmd = "curl -L --insecure http://" + ip + ":" + port + "/name.html"
					fmt.Println(cmd)
					res, _ := RunCommand(cmd)
					fmt.Println(res)
					Expect(res).Should(ContainSubstring("test-loadbalancer"), func() string { return res })
				}
			})

			It("Validate Daemonset", func() {
				if *destroy {
					return
				}
				fmt.Println("Validating Daemonset")
				nodes := ParseNode(kubeconfig, false)
				pods := ParsePod(kubeconfig, false)
				count := CountOfStringInSlice("test-daemonset", pods)
				fmt.Println("POD COUNT")
				fmt.Println(count)
				fmt.Println("NODE COUNT")
				fmt.Println(len(nodes))
				Eventually(len(nodes)).Should((Equal(count)), "120s", "60s")
			})

			It("Validate Ingress", func() {
				if *destroy {
					return
				}
				fmt.Println("Validating Ingress")

				ingressIps := FetchIngressIP(kubeconfig)
				for _, ip := range ingressIps {
					cmd := "curl  --header host:foo1.bar.com" + " http://" + ip + "/name.html"
					fmt.Println(cmd)
					//Access path from outside node
					res, _ := RunCommand(cmd)
					fmt.Println(res)
					Eventually(res).Should((ContainSubstring("test-ingress")), "120s", "60s", func() string { return res })
				}
			})
			It("Validate Local Path Provisioner storage ", func() {
				if *destroy {
					return
				}
				fmt.Println("Validating Local Path Provisioner")
				cmd := "kubectl get pvc local-path-pvc --kubeconfig=" + kubeconfig
				res, _ := RunCommand(cmd)
				fmt.Println(res)
				Eventually(res).Should((ContainSubstring("local-path-pvc")), "120s", "60s")
				Eventually(res).Should((ContainSubstring("Bound")), "120s", "60s")

				cmd = "kubectl get pod volume-test --kubeconfig=" + kubeconfig
				res, _ = RunCommand(cmd)
				fmt.Println(res)

				Eventually(res).Should((ContainSubstring("volume-test")), "120s", "60s", func() string { return res })

				cmd = "kubectl --kubeconfig=" + kubeconfig + " exec volume-test -- sh -c 'echo local-path-test > /data/test'"
				fmt.Println(cmd)
				res, _ = RunCommand(cmd)
				fmt.Println(res)
				fmt.Println("Data stored", res)

				cmd = "kubectl delete pod volume-test --kubeconfig=" + kubeconfig
				res, _ = RunCommand(cmd)
				fmt.Println(res)
				resource_dir := "./amd64_resource_files"
				cmd = "kubectl apply -f " + resource_dir + "/local-path-provisioner.yaml --kubeconfig=" + kubeconfig
				res, _ = RunCommand(cmd)
				fmt.Println(res)

				time.Sleep(1 * time.Minute)
				cmd = "kubectl exec volume-test cat /data/test --kubeconfig=" + kubeconfig
				res, _ = RunCommand(cmd)
				fmt.Println("Data after re-creation", res)

				Eventually(res).Should((ContainSubstring("local-path-test")), "120s", "60s", func() string { return res })
			})
			It("Validate DNS Resolution", func() {
				if *destroy {
					return
				}
				fmt.Println("Validating DNS Resolution")
				cmd := "kubectl --kubeconfig=" + kubeconfig + " exec -i -t dnsutils -- nslookup kubernetes.default"
				fmt.Println(cmd)
				res, _ := RunCommand(cmd)
				fmt.Println("Result", res)
				Eventually(res).ShouldNot((ContainSubstring("nslookup")), "120s", "60s")
			})
		})
	})

})

var _ = AfterSuite(func() {
	kubeconfig, masterIPs, workerIPs = BuildCluster(*nodeOs, *clusterType, *externalDb, *resourceName, &testing.T{}, true)
})
