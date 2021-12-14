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
	nodeOs      = "generic/ubuntu2004"
	serverCount = 3
	agentCount  = 2
)

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

var _ = Describe("Test:", func() {
	Context("Verify Cluster Creation", func() {
		It("Creates the Cluster", func() {
			var err error
			serverNodeNames, agentNodeNames, err = e2e.CreateCluster(nodeOs, serverCount, agentCount)
			Expect(err).NotTo(HaveOccurred())
			fmt.Println("CLUSTER Config")
			fmt.Println("OS:", nodeOs)
			fmt.Println("Server Nodes:", serverNodeNames)
			fmt.Println("Agent Nodes:", agentNodeNames)
		})
		It("Verify Node and Pod Status", func() {
			var err error
			kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())

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
