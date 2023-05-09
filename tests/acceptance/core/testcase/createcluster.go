package testcase

import (
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	util2 "github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/ginkgo/v2"

	"github.com/onsi/gomega"
)

func TestBuildCluster(g ginkgo.GinkgoTInterface, destroy bool) {
	status, err := factory.BuildCluster(g, false)

	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(status).To(gomega.Equal("cluster created"))

	if strings.Contains(util2.ClusterType, "etcd") {
		fmt.Println("Backend:", util2.ClusterType)
	} else {
		fmt.Println("Backend:", util2.ExternalDb)
	}

	if util2.ExternalDb != "" && util2.ClusterType == "" {
		for i := 0; i > len(util2.ServerIPs); i++ {
			cmd := "grep \"datastore-endpoint\" /etc/systemd/system/k3s.service"
			res, err := util2.RunCmdOnNode(cmd, string(util2.ServerIPs[0]))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(res).Should(gomega.ContainSubstring(util2.RenderedTemplate))
		}
	}

	util2.PrintFileContents(util2.KubeConfigFile)
	gomega.Expect(util2.KubeConfigFile).ShouldNot(gomega.BeEmpty())
	gomega.Expect(util2.ServerIPs).ShouldNot(gomega.BeEmpty())

	fmt.Println("Server Node IPS:", util2.ServerIPs)
	fmt.Println("Agent Node IPS:", util2.AgentIPs)

	if util2.NumAgents > 0 {
		gomega.Expect(util2.AgentIPs).ShouldNot(gomega.BeEmpty())
	} else {
		gomega.Expect(util2.AgentIPs).Should(gomega.BeEmpty())
	}
}

// TestNodeStatus test the status of the nodes in the cluster using 2 custom assert functions
func TestNodeStatus(
	g ginkgo.GinkgoTInterface,
	nodeAssertReadyStatus assert.NodeAssertFunc,
	nodeAssertVersion assert.NodeAssertFunc,
) {
	fmt.Printf("\nFetching node status\n")

	expectedNodeCount := util2.NumServers + util2.NumAgents

	gomega.Eventually(func(g gomega.Gomega) {
		nodes, err := util2.ParseNodes(false)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(len(nodes)).To(gomega.Equal(expectedNodeCount),
			"Number of nodes should match the spec")

		for _, node := range nodes {
			if nodeAssertReadyStatus != nil {
				nodeAssertReadyStatus(g, node)
			}
			if nodeAssertVersion != nil {
				nodeAssertVersion(g, node)
			}
		}
	}, "600s", "5s").Should(gomega.Succeed())
}

// TestPodStatus test the status of the pods in the cluster using 2 custom assert functions
func TestPodStatus(
	g ginkgo.GinkgoTInterface,
	podAssertRestarts assert.PodAssertFunc,
	podAssertReady assert.PodAssertFunc,
	podAssertStatus assert.PodAssertFunc,

) {
	fmt.Printf("\nFetching pod status\n")

	gomega.Eventually(func(g gomega.Gomega) {
		pods, err := util2.ParsePods(false)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		for _, pod := range pods {
			if strings.Contains(pod.Name, "helm-install") {
				g.Expect(pod.Status).Should(gomega.Equal("Completed"), pod.Name)
			} else if strings.Contains(pod.Name, "apply") {
				g.Expect(pod.Status).Should(gomega.SatisfyAny(
					gomega.ContainSubstring("Error"),
					gomega.Equal("Completed"),
				), pod.Name)
			} else {
				g.Expect(pod.Status).Should(gomega.Equal("Running"), pod.Name)
				if podAssertRestarts != nil {
					podAssertRestarts(g, pod)
				}
				if podAssertReady != nil {
					podAssertReady(g, pod)
				}
				if podAssertStatus != nil {
					podAssertStatus(g, pod)
				}
			}
		}
	}, "600s", "5s").Should(gomega.Succeed())
}
