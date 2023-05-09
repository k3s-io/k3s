package assert

import (
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/gomega"
)

// NodeAssertFunc is a function type used to create node assertions
type NodeAssertFunc func(g gomega.Gomega, node util.Node)

// NodeAssertVersionUpgraded  custom assertion func that asserts that node
// is upgraded to the specified version
func NodeAssertVersionUpgraded() NodeAssertFunc {
	return func(g gomega.Gomega, node util.Node) {
		g.Expect(node.Version).Should(gomega.Equal(*util.UpgradeVersion),
			"Nodes should all be upgraded to the specified version", node.Name)
	}
}

// NodeAssertReadyStatus custom assertion func that asserts that node is Ready
func NodeAssertReadyStatus() NodeAssertFunc {
	return func(g gomega.Gomega, node util.Node) {
		g.Expect(node.Status).Should(gomega.Equal("Ready"),
			"Nodes should all be in Ready state")
	}
}

// NodeAssertCount custom assertion func that asserts that node count is as expected
func NodeAssertCount() NodeAssertFunc {
	return func(g gomega.Gomega, node util.Node) {
		expectedNodeCount := util.NumServers + util.NumAgents
		nodes, err := util.ParseNodes(false)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(len(nodes)).To(gomega.Equal(expectedNodeCount),
			"Number of nodes should match the spec")
	}
}

// CheckComponentCmdNode runs a command on a node and asserts that the value received
// contains the specified substring
func CheckComponentCmdNode(cmd string, ip string, assert string) {
	gomega.Eventually(func(g gomega.Gomega) {
		res, err := util.RunCmdOnNode(cmd, ip)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(res).Should(gomega.ContainSubstring(assert))
	}, "420s", "5s").Should(gomega.Succeed())
}
