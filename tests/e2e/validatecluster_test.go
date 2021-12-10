package e2e

import (
	"fmt"
	"flag"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
	"testing"
)

func Test_E2EClusterValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Create Cluster Test Suite")
}

var nodeOs = flag.String("nodeOs", "generic/ubuntu2004", "a string")
var serverCount = flag.Int ("serverCount", 3, "an integer")
var agentCount = flag.Int("agentCount", 2, "an intege")

var _ = Describe("Test:", func() {
	Context("Create Cluster:", func() {
		Context("Verify Cluster Creation", func() {
			It("Verify Node and Pod Status", func() {

				serverNodenames := make([]string, 0, 10)
				agentNodenames := make([]string, 0, 10)
				Kubeconfig, serverNodenames, agentNodenames, err = CreateCluster(*nodeOs, *serverCount, *agentCount, &testing.T{})
				fmt.Println("CLUSTER Config")
				fmt.Println("OS:", *nodeOs)
				fmt.Println("Server Nodes:", serverNodenames)
				fmt.Println("Agent Nodes:", agentNodenames)

				cmd := "vagrant ssh server-0 -c  \"ip -f inet addr show eth1| awk '/inet / {print $2}'|cut -d/ -f1\""
				ipaddr, _ := RunCommand(cmd)
				ips := strings.Trim(ipaddr, "")
				ip := strings.Split(ips, "inet")
				Kubeconfig = strings.Replace(Kubeconfig, "127.0.0.1", strings.TrimSpace(ip[1]), 1)
				fmt.Println("KubeConfig\n", Kubeconfig)
				kubeconfigFile := "kubeconfig"
				WriteToFile(Kubeconfig, kubeconfigFile)

				fmt.Printf("\nFetching node status\n")
				nodes := ParseNode(kubeconfigFile, true)
				for _, config := range nodes {
					Expect(config.Status).Should(Equal("Ready"), func() string { return config.Name })
				}
				fmt.Printf("\nFetching Pods status\n")

				pods := ParsePod(kubeconfigFile, true)
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						Expect(pod.Status).Should(Equal("Completed"), func() string { return pod.Name })
					} else {
						Expect(pod.Status).Should(Equal("Running"), func() string { return pod.Name })
					}
				}
			})

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
		cmd := "vagrant destroy -f"
		_, err = RunCommand(cmd)
	}
})
