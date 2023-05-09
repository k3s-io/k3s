package testcase

import (
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/ginkgo/v2"

	"github.com/onsi/gomega"
)

func TestBuildCluster(g ginkgo.GinkgoTInterface, destroy bool) {
	status, err := factory.BuildCluster(g, false)

	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(status).To(gomega.Equal("cluster created"))

	if strings.Contains(util.ClusterType, "etcd") {
		fmt.Println("Backend:", util.ClusterType)
	} else {
		fmt.Println("Backend:", util.ExternalDb)
	}

	if util.ExternalDb != "" && util.ClusterType == "" {
		for i := 0; i > len(util.ServerIPs); i++ {
			cmd := "grep \"datastore-endpoint\" /etc/systemd/system/k3s.service"
			res, err := util.RunCmdOnNode(cmd, string(util.ServerIPs[0]))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(res).Should(gomega.ContainSubstring(util.RenderedTemplate))
		}
	}

	util.PrintFileContents(util.KubeConfigFile)
	gomega.Expect(util.KubeConfigFile).ShouldNot(gomega.BeEmpty())
	gomega.Expect(util.ServerIPs).ShouldNot(gomega.BeEmpty())

	fmt.Println("Server Node IPS:", util.ServerIPs)
	fmt.Println("Agent Node IPS:", util.AgentIPs)

	if util.NumAgents > 0 {
		gomega.Expect(util.AgentIPs).ShouldNot(gomega.BeEmpty())
	} else {
		gomega.Expect(util.AgentIPs).Should(gomega.BeEmpty())
	}
}

// TestNodeStatus test the status of the nodes in the cluster using 2 custom assert functions
func TestNodeStatus(
	g ginkgo.GinkgoTInterface,
	nodeAssertReadyStatus assert.NodeAssertFunc,
	nodeAssertVersion assert.NodeAssertFunc,
) {
	fmt.Printf("\nFetching node status\n")

	expectedNodeCount := util.NumServers + util.NumAgents

	gomega.Eventually(func(g gomega.Gomega) {
		nodes, err := util.ParseNodes(false)
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
		pods, err := util.ParsePods(false)
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
