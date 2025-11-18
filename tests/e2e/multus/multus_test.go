// This test verifies that two nodes, which can't connect using the local network, are
// able to still connect using the node-external-ip. In real life, node-external-ip
// would be a public IP. In the test, we create two networks, one sets the node
// internal-ip and the other sets the node-external-ip. Traffic is blocked on the former

package multus

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// json structure stored in metadata.annotations.k8s\.v1\.cni\.cncf\.io\/network-status
type NetworkConfig struct {
	Name      string         `json:"name"`
	Interface string         `json:"interface,omitempty"`
	IPs       []string       `json:"ips"`
	Default   bool           `json:"default,omitempty"`
	MAC       string         `json:"mac,omitempty"`
	DNS       map[string]any `json:"dns,omitempty"`
}

const successMessage = "5 packets transmitted, 5 received, 0% packet loss"

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 1, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// getMultusIp returns the IP address on the multus network of the multus-demo pod running on nodeName
func getMultusIp(kubeConfigFile, nodeName string) (string, error) {
	cmd := `kubectl get pods -l app=multus-demo --field-selector spec.nodeName=` + nodeName + ` -o jsonpath='{.items[0]..metadata.annotations.k8s\.v1\.cni\.cncf\.io\/network-status}'  --kubeconfig=` + kubeConfigFile
	res, err := e2e.RunCommand(cmd)
	if err != nil {
		return "", err
	}

	var networkStatus []NetworkConfig

	err = json.Unmarshal([]byte(res), &networkStatus)
	if err != nil {
		return "", err
	}

	fmt.Printf("network status: %v\n", networkStatus)

	return networkStatus[1].IPs[0], nil
}

func pingOverMultusNetwork(kubeConfigFile, sourceNodeName, destMultusIP string) (bool, error) {
	//get the name of the multus-demo pod
	cmd := `kubectl get pods -l app=multus-demo --field-selector spec.nodeName=` + sourceNodeName + ` -o jsonpath='{.items[0].metadata.name}'  --kubeconfig=` + kubeConfigFile
	podName, err := e2e.RunCommand(cmd)
	if err != nil {
		return false, err
	}
	fmt.Println(podName)

	//run ping command in that pod
	cmd = `kubectl exec --kubeconfig=` + kubeConfigFile + ` ` + podName + ` -- ping -c 5 ` + destMultusIP
	res, err := e2e.RunCommand(cmd)
	fmt.Println(res)
	if err != nil {
		return false, err
	}

	//run curl command
	podCmd := `curl -m 5 -s -f http://` + destMultusIP + `:1180`

	cmd = `kubectl exec --kubeconfig=` + kubeConfigFile + ` ` + podName + ` -- ` + podCmd
	res, err = e2e.RunCommand(cmd)
	fmt.Println(res)
	if err != nil {
		return false, err
	}
	return strings.Contains(res, successMessage), nil
}

func pingBetweenNode(src, dest e2e.VagrantNode) (bool, error) {
	srcIP, err := src.FetchNodeSecondaryIP()
	if err != nil {
		return false, err
	}
	fmt.Printf("srcIP: %s\n", srcIP)

	destIP, err := dest.FetchNodeSecondaryIP()
	if err != nil {
		return false, err
	}
	fmt.Printf("destIP: %s\n", destIP)
	cmd := "ping -c 5 " + destIP
	output, err := src.RunCmdOnNode(cmd)
	fmt.Printf("output: %s\n", output)
	return strings.Contains(output, successMessage), nil

}

func runRandomTests(kubeConfigFile, nodeName string) (bool, error) {
	//get the name of the multus-demo pod
	cmd := `kubectl get pods -l app=multus-demo --field-selector spec.nodeName=` + nodeName + ` -o jsonpath='{.items[0].metadata.name}'  --kubeconfig=` + kubeConfigFile
	podName, err := e2e.RunCommand(cmd)
	if err != nil {
		fmt.Printf("error: %s\n", err)
	}
	fmt.Println(podName)

	cmd = `kubectl exec --kubeconfig=` + kubeConfigFile + ` ` + podName + ` -- ip a`
	res, err := e2e.RunCommand(cmd)
	fmt.Printf("res: %s", res)
	if err != nil {
		fmt.Printf("error: %s\n", err)
	}

	cmd = `kubectl get pods -o wide -A  --kubeconfig=` + kubeConfigFile
	res, err = e2e.RunCommand(cmd)
	fmt.Printf("res: %s", res)
	if err != nil {
		fmt.Printf("error: %s\n", err)
	}
	return true, nil
}

func Test_E2EMultus(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Multus config Suite", suiteConfig, reporterConfig)

}

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Multus config", Ordered, func() {
	Context("Cluster comes up with Multus enabled", func() {
		It("Starts up with no issues", func() {
			var err error
			if *local {
				tc, err = e2e.CreateLocalCluster(*nodeOS, *serverCount, *agentCount)
			} else {
				tc, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
			}
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			By("CLUSTER CONFIG")
			By("OS: " + *nodeOS)
			By(tc.Status())
		})

		It("Checks Node Status", func() {
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, e2e.VagrantSlice(tc.AllNodes()))
			}, "620s", "5s").Should(Succeed())
			e2e.DumpNodes(tc.KubeconfigFile)
		})

		It("Checks pod status", func() {
			By("Fetching pod status")
			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "620s", "10s").Should(Succeed())
		})
	})
	Context("Deploy workloads to check cluster connectivity of the nodes", func() {
		It("Verifies that each node has vagrant IP", func() {
			nodeIPs, err := e2e.GetNodeIPs(tc.KubeconfigFile)
			fmt.Printf("nodeIPs: %v", nodeIPs)
			Expect(err).NotTo(HaveOccurred())
			for _, node := range nodeIPs {
				Expect(node.IPv4).Should(ContainSubstring("10.10."))
			}
		})
		It("Verifies that each pod has vagrant IP or clusterCIDR IP", func() {
			podIPs, err := e2e.GetPodIPs(tc.KubeconfigFile)
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range podIPs {
				Expect(pod.IPv4).Should(Or(ContainSubstring("10.10."), ContainSubstring("10.42.")), pod.Name)
			}
		})
		// It("Verifies that nodes can ping each other on secondary network", func() {
		// 	result, err := pingBetweenNode(tc.Servers[0], tc.Agents[0])
		// 	Expect(err).NotTo(HaveOccurred())
		// 	Expect(result).To(Equal(true))
		// 	result, err = pingBetweenNode(tc.Agents[0], tc.Servers[0])
		// 	Expect(err).NotTo(HaveOccurred())
		// 	Expect(result).To(Equal(true))
		// })
		It("Deploys Multus NetworkAttachmentDefinition", func() {
			_, err := tc.DeployWorkload("multus_network_attach.yaml")
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(5 * time.Second)
		})
		It("Verifies internode connectivity over multus network", func() {
			_, err := tc.DeployWorkload("multus_pod_client.yaml")
			Expect(err).NotTo(HaveOccurred())

			// Wait for each multus-demo pod to have an IP address on the multus network
			// then store them
			multusIPs := map[string]string{"server-0": "", "agent-0": ""}

			for nodename := range multusIPs {
				Eventually(func(g Gomega) {
					multusIp, err := getMultusIp(tc.KubeconfigFile, nodename)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(multusIp).Should(ContainSubstring("172.17.0"), "multus IP: "+multusIp)
					multusIPs[nodename] = multusIp
				}, "40s", "5s").Should(Succeed(), "failed to get Multus IP for node "+nodename)
			}

			Eventually(func(g Gomega) {
				// result, err := runRandomTests(tc.KubeconfigFile, "server-0")
				// g.Expect(err).NotTo(HaveOccurred())
				// g.Expect(result).To(Equal(true))
				//ping pod on agent-0 from pod on server-0
				res, err := pingOverMultusNetwork(tc.KubeconfigFile, "server-0", multusIPs["agent-0"])
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).To(Equal(true))
				//ping pod on server-0 from pod on agent-0
				res, err = pingOverMultusNetwork(tc.KubeconfigFile, "agent-0", multusIPs["server-0"])
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).To(Equal(true))

			}, "40s", "5s").Should(Succeed(), "failed to ping between pods on multus network")

			// for _, ip := range clientIPs {
			// 	cmd := "kubectl exec svc/client-curl -- curl -m 5 -s -f http://" + ip.IPv4 + "/name.html"
			// 	Eventually(func() (string, error) {
			// 		return e2e.RunCommand(cmd)
			// 	}, "30s", "10s").Should(ContainSubstring("client-deployment"), "failed cmd: "+cmd)
			// }
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		Expect(e2e.SaveJournalLogs(tc.AllNodes())).To(Succeed())
		Expect(e2e.TailPodLogs(50, tc.AllNodes())).To(Succeed())
	} else {
		Expect(e2e.GetCoverageReport(tc.AllNodes())).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
