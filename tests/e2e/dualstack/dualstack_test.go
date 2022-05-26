package validatecluster

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

// Valid nodeOS: generic/ubuntu2004, opensuse/Leap-15.3.x86_64
var nodeOS = flag.String("nodeOS", "generic/ubuntu2004", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 0, "number of agent nodes")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s1 or nil for latest commit from master

type objIP struct {
	name string
	ipv4 string
	ipv6 string
}

func getPodIPs(kubeConfigFile string) ([]objIP, error) {
	cmd := `kubectl get pods -A -o=jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.podIPs[*].ip}{"\n"}{end}' --kubeconfig=` + kubeConfigFile
	return getObjIPs(cmd)
}
func getNodeIPs(kubeConfigFile string) ([]objIP, error) {
	cmd := `kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.addresses[?(@.type == "InternalIP")].address}{"\n"}{end}' --kubeconfig=` + kubeConfigFile
	return getObjIPs(cmd)
}

func getObjIPs(cmd string) ([]objIP, error) {
	var objIPs []objIP
	res, err := e2e.RunCommand(cmd)
	if err != nil {
		return nil, err
	}
	objs := strings.Split(res, "\n")
	objs = objs[:len(objs)-1]

	for _, obj := range objs {
		fields := strings.Fields(obj)
		if len(fields) > 2 {
			objIPs = append(objIPs, objIP{name: fields[0], ipv4: fields[1], ipv6: fields[2]})
		} else if len(fields) > 1 {
			objIPs = append(objIPs, objIP{name: fields[0], ipv4: fields[1]})
		} else {
			objIPs = append(objIPs, objIP{name: fields[0]})
		}
	}
	return objIPs, nil
}

func Test_E2EDualStack(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validate DualStack Suite")
}

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

var _ = Describe("Verify DualStack Configuration", func() {

	It("Starts up with no issues", func() {
		var err error
		serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
		Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog())
		fmt.Println("CLUSTER CONFIG")
		fmt.Println("OS:", *nodeOS)
		fmt.Println("Server Nodes:", serverNodeNames)
		fmt.Println("Agent Nodes:", agentNodeNames)
		kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred())
	})

	It("Checks Node Status", func() {
		Eventually(func(g Gomega) {
			nodes, err := e2e.ParseNodes(kubeConfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, node := range nodes {
				g.Expect(node.Status).Should(Equal("Ready"))
			}
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParseNodes(kubeConfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Checks Pod Status", func() {
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
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParsePods(kubeConfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Verifies that each node has IPv4 and IPv6", func() {
		nodeIPs, err := getNodeIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, node := range nodeIPs {
			Expect(node.ipv4).Should(ContainSubstring("10.10.10"))
			Expect(node.ipv6).Should(ContainSubstring("a11:decf:c0ff"))
		}
	})
	It("Verifies that each pod has IPv4 and IPv6", func() {
		podIPs, err := getPodIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, pod := range podIPs {
			Expect(pod.ipv4).Should(Or(ContainSubstring("10.10.10"), ContainSubstring("10.42.")), pod.name)
			Expect(pod.ipv6).Should(Or(ContainSubstring("a11:decf:c0ff"), ContainSubstring("2001:cafe:42")), pod.name)
		}
	})

	It("Verifies ClusterIP Service", func() {
		_, err := e2e.DeployWorkload("dualstack_clusterip.yaml", kubeConfigFile, false)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() (string, error) {
			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
			return e2e.RunCommand(cmd)
		}, "120s", "5s").Should(ContainSubstring("ds-clusterip-pod"))

		// Checks both IPv4 and IPv6
		clusterips, err := e2e.FetchClusterIP(kubeConfigFile, "ds-clusterip-svc", true)
		Expect(err).NotTo(HaveOccurred())
		for _, ip := range strings.Split(clusterips, ",") {
			if strings.Contains(ip, "::") {
				ip = "[" + ip + "]"
			}
			pods, err := e2e.ParsePods(kubeConfigFile, false)
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods {
				if !strings.HasPrefix(pod.Name, "ds-clusterip-pod") {
					continue
				}
				cmd := fmt.Sprintf("curl -L --insecure http://%s", ip)
				Eventually(func() (string, error) {
					return e2e.RunCmdOnNode(cmd, serverNodeNames[0])
				}, "60s", "5s").Should(ContainSubstring("Welcome to nginx!"), "failed cmd: "+cmd)
			}
		}
	})
	It("Verifies Ingress", func() {
		_, err := e2e.DeployWorkload("dualstack_ingress.yaml", kubeConfigFile, false)
		Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")
		cmd := "kubectl get ingress ds-ingress --kubeconfig=" + kubeConfigFile + " -o jsonpath=\"{.spec.rules[*].host}\""
		hostName, err := e2e.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		nodeIPs, err := getNodeIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		for _, node := range nodeIPs {
			cmd := fmt.Sprintf("curl  --header host:%s http://%s/name.html", hostName, node.ipv4)
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "10s", "2s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
			cmd = fmt.Sprintf("curl  --header host:%s http://[%s]/name.html", hostName, node.ipv6)
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "5s", "1s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
		}
	})

	It("Verifies NodePort Service", func() {
		_, err := e2e.DeployWorkload("dualstack_nodeport.yaml", kubeConfigFile, false)
		Expect(err).NotTo(HaveOccurred())
		cmd := "kubectl get service ds-nodeport-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
		nodeport, err := e2e.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		nodeIPs, err := getNodeIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, node := range nodeIPs {
			cmd = "curl -L --insecure http://" + node.ipv4 + ":" + nodeport + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "10s", "1s").Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
			cmd = "curl -L --insecure http://[" + node.ipv6 + "]:" + nodeport + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "10s", "1s").Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
		}
	})

})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentGinkgoTestDescription().Failed
})

var _ = AfterSuite(func() {
	if failed {
		fmt.Println("FAILED!")
	} else {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
