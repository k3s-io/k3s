package testcase

import (
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	"github.com/k3s-io/k3s/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBuildCluster(g GinkgoTInterface) {
	cluster := factory.GetCluster(g)
	Expect(cluster.Status).To(Equal("cluster created"))

	if strings.Contains(cluster.ClusterType, "etcd") {
		fmt.Println("Backend:", cluster.ClusterType)
	} else {
		fmt.Println("Backend:", cluster.ExternalDb)
	}

	if cluster.ExternalDb != "" && cluster.ClusterType == "" {
		for i := 0; i > len(cluster.ServerIPs); i++ {
			cmd := "grep \"datastore-endpoint\" /etc/systemd/system/k3s.service"
			res, err := shared.RunCmdOnNode(cmd, string(cluster.ServerIPs[0]))
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring(cluster.RenderedTemplate))
		}
	}

	err := shared.PrintFileContents(shared.KubeConfigFile)
	if err != nil {
		return
	}

	Expect(shared.KubeConfigFile).ShouldNot(BeEmpty())
	Expect(cluster.ServerIPs).ShouldNot(BeEmpty())

	fmt.Println("Server Node IPS:", cluster.ServerIPs)
	fmt.Println("Agent Node IPS:", cluster.AgentIPs)

	if cluster.NumAgents > 0 {
		Expect(cluster.AgentIPs).ShouldNot(BeEmpty())
	} else {
		Expect(cluster.AgentIPs).Should(BeEmpty())
	}
}
