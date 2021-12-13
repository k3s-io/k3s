package e2e
import (
	"flag"
	"fmt"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	"strings"
	"testing"
	"time"
)

var upgradeVersion = flag.String("upgrade_version", "", "a string")


func Test_E2EClusterUpgradeValidation(t *testing.T) {
	reporters := []Reporter{
		reporters.NewJUnitReporter("./" + *resourceName + "upgraderesults.xml"),
	}
	RegisterFailHandler(Fail)
	RunSpecsWithCustomReporters(t, "Cluster Upgrade Validation", reporters)
}

var _ = Describe("Test: ", func() {

	Context("Cluster Upgrade" + *nodeOs+ " " + *clusterType+ " " + *externalDb, func() {

		It("Verify Node and Pod Status", func() {
			kubeconfig, masterIPs, workerIPs = BuildCluster(*nodeOs, *clusterType, *externalDb, *resourceName, &testing.T{}, *destroy)
			if *destroy {
				fmt.Printf("\nCluster is being Deleted\n")
				return
			}
			fmt.Println("Cluster Version", *upgradeVersion)
			fmt.Println("\nCluster Config:\nOS", *nodeOs, "Backend", *clusterType, *externalDb)
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

		It("Validate ClusterIP", func() {
			if *destroy {
				return
			}
			DeployWorkloads(*arch, kubeconfig)
			fmt.Println("Validating ClusterIP")
			clusterip := FetchClusterIP(kubeconfig, "nginx-clusterip-svc")
			cmd := "curl -L --insecure http://" + clusterip + "/name.html"
			fmt.Println(cmd)
			//Fetch External IP to login to node and validate cluster ip
			node_external_ip := FetchNodeExternalIP(kubeconfig)

			for _, ip := range node_external_ip {
				res := RunCmdOnNode(cmd, ip, *sshuser, *sshkey)
				fmt.Println(res)
				Expect(res).Should(ContainSubstring("test-clusterip"), func() string { return res })
			}
		})

		It("\nValidate NodePort", func() {
			if *destroy {
				return
			}
			fmt.Println("Validating NodePort")
			node_external_ip := FetchNodeExternalIP(kubeconfig)
			cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + kubeconfig + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport,_ := RunCommand(cmd)
			for _, nodeExternalIp := range node_external_ip {
				cmd := "curl -L --insecure http://" + nodeExternalIp + ":" + nodeport + "/name.html"
				fmt.Println(cmd)
				res,_ := RunCommand(cmd)
				fmt.Println(res)
				Expect(res).Should(ContainSubstring("test-nodeport"), func() string { return res }) //Need to check of returned value is unique to node
			}
		})

		It("\nValidate LoadBalancer", func() {
			if *destroy {
				return
			}
			fmt.Println("Validating Service LoaadBalancer")
			node_external_ip := FetchNodeExternalIP(kubeconfig)
			cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + kubeconfig + " --output jsonpath=\"{.spec.ports[0].port}\""
			port, _:= RunCommand(cmd)
			for _, ip := range node_external_ip {
				cmd = "curl -L --insecure http://" + ip + ":" + port + "/name.html"
				fmt.Println(cmd)
				res,_ := RunCommand(cmd)
				fmt.Println(res)
				Expect(res).Should(ContainSubstring("test-loadbalancer"), func() string { return res })
			}
		})

		It("\nValidate Daemonset", func() {
			if *destroy {
				return
			}
			fmt.Println("Validating Daemonset")
			nodes := ParseNode(kubeconfig, false) //nodes :=
			pods := ParsePod(kubeconfig, false)
			fmt.Println("\nValidating Daemonset")
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
			for _, ip := range ingressIps{
				cmd := "curl  --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				fmt.Println(cmd)
				//Access path from outside node
				res, _ := RunCommand(cmd)
				fmt.Println(res)
				Eventually(res).Should((ContainSubstring("test-ingress")), "120s", "60s", func() string { return res })
			}
		})

		It("\nValidate Local Path Provisioner storage ", func() {
			if *destroy {
				return
			}
			fmt.Println("Validating Local Path Provisioner")
			cmd := "kubectl get pvc local-path-pvc --kubeconfig=" + kubeconfig
			res,_ := RunCommand(cmd)
			fmt.Println(res)
			Eventually(res).Should((ContainSubstring("local-path-pvc")), "120s", "60s")
			Eventually(res).Should((ContainSubstring("Bound")), "120s", "60s")

			cmd = "kubectl get pod volume-test --kubeconfig=" + kubeconfig
			res,_ = RunCommand(cmd)
			fmt.Println(res)

			Eventually(res).Should((ContainSubstring("volume-test")), "120s", "60s", func() string { return res })

			cmd = "kubectl --kubeconfig=" + kubeconfig + " exec volume-test -- sh -c 'echo local-path-test > /data/test'"
			res, _= RunCommand(cmd)
			fmt.Println(res)
			fmt.Println("Data stored", res)

			cmd = "kubectl delete pod volume-test --kubeconfig=" + kubeconfig
			res,_ = RunCommand(cmd)
			fmt.Println(res)
			resource_dir := "./amd64_resource_files"
			cmd = "kubectl apply -f " + resource_dir + "/local-path-provisioner.yaml --kubeconfig=" + kubeconfig
			res, _= RunCommand(cmd)
			fmt.Println(res)

			time.Sleep(1 * time.Minute)
			cmd = "kubectl exec volume-test cat /data/test --kubeconfig=" + kubeconfig
			res,_ = RunCommand(cmd)
			fmt.Println("Data after re-creation", res)

			Eventually(res).Should((ContainSubstring("local-path-test")), "120s", "60s", func() string { return res })
		})

		It("\nVerify Cluster is upgraded and default pods running", func() {
			if *destroy {
				//fmt.Printf("\nCluster is Deleted\n")
				return
			}
			MIPs := strings.Split(masterIPs,",")

			for _, ip := range MIPs {
			    cmd := "sudo sed -i \"s/|/| INSTALL_K3S_VERSION=" + *upgradeVersion + "/g\" /tmp/master_cmd"
				fmt.Println(cmd)
				_ = RunCmdOnNode(cmd,ip, *sshuser, *sshkey)
				cmd =  "sudo chmod u+x /tmp/master_cmd && sudo /tmp/master_cmd"
				_ = RunCmdOnNode(cmd, ip, *sshuser, *sshkey)
			}

			WIPs := strings.Split(workerIPs,",")
			for i := 0; i < len(WIPs) && len(WIPs[0])>1; i++ {
				ip := WIPs[i]
				strings.TrimSpace(WIPs[i])
				cmd := "sudo sed -i \"s/|/| INSTALL_K3S_VERSION=" + *upgradeVersion + "/g\" /tmp/agent_cmd"
				_ = RunCmdOnNode(cmd,ip, *sshuser, *sshkey)
				By("Step4")
				cmd =  "sudo chmod u+x /tmp/agent_cmd && sudo /tmp/agent_cmd"
				_ = RunCmdOnNode(cmd, ip, *sshuser, *sshkey)
			}

			time.Sleep(5 * time.Second)
			fmt.Println("After Upgrade")
			nodes := ParseNode(kubeconfig, true)
			for _, config := range nodes {
				Expect(config.Status).Should(Equal("Ready"))
			}
			pods := ParsePod(kubeconfig, true)
			for _, pod := range pods {
				if strings.Contains(pod.Name, "helm-install") {
					Expect(pod.Status).Should(Equal("Completed"))
				} else {
					Expect(pod.Status).Should(Equal("Running"))
				}
			}
		})


		It("Validate ClusterIP after upgrade", func() {
			if *destroy {
				return
			}
			fmt.Println("Validating ClusterIP")
			clusterip := FetchClusterIP(kubeconfig, "nginx-clusterip-svc")
			cmd := "curl -L --insecure http://" + clusterip + "/name.html"
			fmt.Println(cmd)
			//Fetch External IP to login to node and validate cluster ip
			node_external_ip := FetchNodeExternalIP(kubeconfig)

			for _, ip := range node_external_ip {
				res := RunCmdOnNode(cmd, ip, *sshuser, *sshkey)
				fmt.Println(res)
				Expect(res).Should(ContainSubstring("test-clusterip"), func() string { return res })
			}
		})

		It("Validate NodePort after upgrade", func() {
			if *destroy {
				return
			}
			fmt.Println("Validating NodePort")
			node_external_ip := FetchNodeExternalIP(kubeconfig)
			cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + kubeconfig + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport,_ := RunCommand(cmd)
			for _, nodeExternalIp := range node_external_ip {
				cmd := "curl -L --insecure http://" + nodeExternalIp + ":" + nodeport + "/name.html"
				fmt.Println(cmd)
				res,_ := RunCommand(cmd)
				fmt.Println(res)
				Expect(res).Should(ContainSubstring("test-nodeport"), func() string { return res }) //Need to check of returned value is unique to node
			}
		})

		It("\nValidate Service LoadBalancer after upgrade", func() {
			if *destroy {
				return
			}
			fmt.Println("Validating Service LoaadBalancer")
			node_external_ip := FetchNodeExternalIP(kubeconfig)
			cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + kubeconfig + " --output jsonpath=\"{.spec.ports[0].port}\""
			port,_ := RunCommand(cmd)
			for _, ip := range node_external_ip {
				cmd = "curl -L --insecure http://" + ip + ":" + port + "/name.html"
				fmt.Println(cmd)
				res,_ := RunCommand(cmd)
				fmt.Println(res)
				Expect(res).Should(ContainSubstring("test-loadbalancer"), func() string { return res })
			}
		})

		It("\nValidate Daemonset after upgrade", func() {
			if *destroy {
				return
			}
			fmt.Println("Validating Daemonset")
			nodes := ParseNode(kubeconfig, false) //nodes :=
			pods := ParsePod(kubeconfig, false)
			fmt.Println("\nValidating Daemonset")
			count := CountOfStringInSlice("test-daemonset", pods)
			fmt.Println("POD COUNT")
			fmt.Println(count)
			fmt.Println("NODE COUNT")
			fmt.Println(len(nodes))
			Eventually(len(nodes)).Should((Equal(count)), "120s", "60s")
		})
		It("nValidate Ingress after upgrade", func() {
			if *destroy {
				return
			}
			fmt.Println("Validating Ingress")
			node_external_ip := FetchNodeExternalIP(kubeconfig)

			for _, ip := range node_external_ip {
				cmd := "curl  --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				fmt.Println(cmd)
				//Access path from inside node
				res := RunCmdOnNode(cmd, ip, *sshuser, *sshkey)
				fmt.Println(res)
				Eventually(res).Should(ContainSubstring("test-ingress"), "120s", "60s", func() string { return res })
			}

			for _, ip := range node_external_ip {
				cmd := "curl  --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				fmt.Println(cmd)
				//Access path from outside node
				res,_ := RunCommand(cmd)
				fmt.Println(res)
				Eventually(res).Should((ContainSubstring("test-ingress")), "120s", "60s", func() string { return res })
			}
		})
		It("Validating Local Path Provisioner storage after upgrade", func() {
			if *destroy {
				return
			}
			fmt.Println("Validating Local Path Provisioner")
			cmd := "kubectl get pvc local-path-pvc --kubeconfig=" + kubeconfig
			res,_ := RunCommand(cmd)
			fmt.Println(res)
			Eventually(res).Should((ContainSubstring("local-path-pvc")), "120s", "60s")
			Eventually(res).Should((ContainSubstring("Bound")), "120s", "60s")

			cmd = "kubectl get pod volume-test --kubeconfig=" + kubeconfig
			res,_ = RunCommand(cmd)
			fmt.Println(res)

			Eventually(res).Should((ContainSubstring("volume-test")), "120s", "60s", func() string { return res })

			cmd = "kubectl --kubeconfig=" + kubeconfig + " exec volume-test -- sh -c 'echo local-path-test > /data/test'"
			res,_ = RunCommand(cmd)
			fmt.Println(res)
			fmt.Println("Data stored", res)

			cmd = "kubectl delete pod volume-test --kubeconfig=" + kubeconfig
			res,_ = RunCommand(cmd)
			fmt.Println(res)
			resource_dir := "./amd64_resource_files"
			cmd = "kubectl apply -f " + resource_dir + "/local-path-provisioner.yaml --kubeconfig=" + kubeconfig
			res,_ = RunCommand(cmd)
			fmt.Println(res)

			time.Sleep(1 * time.Minute)
			cmd = "kubectl exec volume-test cat /data/test --kubeconfig=" + kubeconfig
			res, _ = RunCommand(cmd)
			fmt.Println("Data after re-creation", res)

			Eventually(res).Should((ContainSubstring("local-path-test")), "120s", "60s", func() string { return res })
		})

		It("Validating dns access after upgrade", func() {
			cmd := "kubectl exec -i -t dnsutils -- nslookup kubernetes.default"

			res, _ := RunCommand(cmd)
			fmt.Println(res)
		})
	})
})
