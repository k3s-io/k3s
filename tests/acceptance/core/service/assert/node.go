package assert

import (
	"fmt"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/customflag"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// NodeAssertFunc is a function type used to create node assertions
type NodeAssertFunc func(g gomega.Gomega, node util.Node)

// NodeAssertVersionTypeUpgrade NodeAssertVersion custom assertion func that asserts that node version is as expected
func NodeAssertVersionTypeUpgrade(installType *customflag.InstallTypeValue) NodeAssertFunc {
	if installType.Version != "" {
		fmt.Printf("Asserting Version: %s\n", installType.Version)
		return func(g gomega.Gomega, node util.Node) {
			g.Expect(node.Version).Should(gomega.Equal(installType.Version),
				"Nodes should all be upgraded to the specified version", node.Name)
		}
	}

	if installType.Commit != "" {
		version := util.GetK3sVersion()
		fmt.Printf("Asserting Commit: %s\n Version: %s", installType.Commit, version)
		return func(g gomega.Gomega, node util.Node) {
			g.Expect(version).Should(gomega.ContainSubstring(node.Version),
				"Nodes should all be upgraded to the specified commit", node.Name)
		}
	}

	return func(g gomega.Gomega, node util.Node) {
		GinkgoT().Errorf("no version or commit specified for upgrade assertion")
	}
}

// NodeAssertReadyStatus custom assertion func that asserts that node is Ready
func NodeAssertReadyStatus() NodeAssertFunc {
	return func(g gomega.Gomega, node util.Node) {
		g.Expect(node.Status).Should(gomega.Equal("Ready"),
			"Nodes should all be in Ready state")
	}
}

// CheckComponentCmdNode runs a command on a node and asserts that the value received
// contains the specified substring.
func CheckComponentCmdNode(cmd, assert, ip string) {
	gomega.Eventually(func(g gomega.Gomega) {
		res, err := util.RunCmdOnNode(cmd, ip)
		if err != nil {
			return
		}
		g.Expect(res).Should(gomega.ContainSubstring(assert))
	}, "420s", "5s").Should(gomega.Succeed())
}

// NodeAssertCount custom assertion func that asserts that node count is as expected
func NodeAssertCount() NodeAssertFunc {
	return func(g gomega.Gomega, node util.Node) {
		expectedNodeCount := util.NumServers + util.NumAgents
		nodes, err := util.ParseNodes(false)
		if err != nil {
			GinkgoT().Logf("Error: %v", err)
		}

		g.Expect(len(nodes)).To(gomega.Equal(expectedNodeCount),
			"Number of nodes should match the spec")
	}
}
