package testcase

import (
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBuildCluster(g GinkgoTInterface, destroy bool) {
	status, err := factory.BuildCluster(g, destroy)
	if err != nil {
		return
	}
	Expect(status).To(Equal("cluster created"))

	if strings.Contains(util.ClusterType, "etcd") {
		fmt.Println("Backend:", util.ClusterType)
	} else {
		fmt.Println("Backend:", util.ExternalDb)
	}

	if util.ExternalDb != "" && util.ClusterType == "" {
		for i := 0; i > len(util.ServerIPs); i++ {
			cmd := "grep \"datastore-endpoint\" /etc/systemd/system/k3s.service"
			res, err := util.RunCmdOnNode(cmd, string(util.ServerIPs[0]))
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring(util.RenderedTemplate))
		}
	}

	util.PrintFileContents(util.KubeConfigFile)
	Expect(util.KubeConfigFile).ShouldNot(BeEmpty())
	Expect(util.ServerIPs).ShouldNot(BeEmpty())

	fmt.Println("Server Node IPS:", util.ServerIPs)
	fmt.Println("Agent Node IPS:", util.AgentIPs)

	if util.NumAgents > 0 {
		Expect(util.AgentIPs).ShouldNot(BeEmpty())
	} else {
		Expect(util.AgentIPs).Should(BeEmpty())
	}
}
