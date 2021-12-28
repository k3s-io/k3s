package validatecluster

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
)

func Test_E2EClusterValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Create Cluster Test Suite", []Reporter{
		reporters.NewJUnitReporter("/tmp/results/junit-cc.xml"),
	})
}

const (
	// Valid nodeOS: generic/ubuntu2004, opensuse/Leap-15.3.x86_64, dweomer/microos.amd64
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
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, true)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"), func() string { return node.Name })
				}
			}, "420s", "5s").Should(Succeed())

			fmt.Printf("\nFetching Pods status\n")
			Eventually(func(g Gomega) {
				pods, err := e2e.ParsePods(kubeConfigFile, true)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"), func() string { return pod.Name })
					} else {
						g.Expect(pod.Status).Should(Equal("Running"), func() string { return pod.Name })
					}
				}
			}, "420s", "5s").Should(Succeed())
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
