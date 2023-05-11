package testcase

import (
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestBuildCluster(g GinkgoTInterface, destroy bool) {
	status, err := factory.BuildCluster(g, destroy)
	if err != nil {
		return
	}
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
