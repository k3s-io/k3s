package validatecluster

import (
	"fmt"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher/k3s/tests/e2e"
)

func Test_E2EClusterValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Create Cluster Test Suite")
}

const (
	// Valid nodeOS: generic/ubuntu2004, opensuse/Leap-15.3.x86_6, dweomer/microos.amd64
	nodeOs      = "generic/ubuntu2004"
	serverCount = 3
	agentCount  = 2
)

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

var _ = Describe("Verify Cluster Creation", func() {
	Context("Create the Cluster", func() {
		It("Starts up with no issues", func() {
			var err error
			serverNodeNames, agentNodeNames, err = e2e.CreateCluster(nodeOs, serverCount, agentCount)
			Expect(err).NotTo(HaveOccurred())
			fmt.Println("CLUSTER CONFIG")
			fmt.Println("OS:", nodeOs)
			fmt.Println("Server Nodes:", serverNodeNames)
			fmt.Println("Agent Nodes:", agentNodeNames)
			kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
		})
		It("Verify Node and Pod Status", func() {
			var err error
			fmt.Printf("\nFetching node status\n")
			nodes, err := e2e.ParseNode(kubeConfigFile, true)
			Expect(err).NotTo(HaveOccurred())
			for _, config := range nodes {
				Expect(config.Status).Should(Equal("Ready"), func() string { return config.Name })
			}
			fmt.Printf("\nFetching Pods status\n")

			pods, err := e2e.ParsePod(kubeConfigFile, true)
			Expect(err).NotTo(HaveOccurred())
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

var failed = false
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
