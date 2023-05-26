package testcase

import (
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"

	. "github.com/onsi/gomega"
)

func TestDaemonset(deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "daemonset.yaml", *util.Arch)
		Expect(err).NotTo(HaveOccurred(),
			"Daemonset manifest not deployed")
	}
	nodes, _ := util.ParseNodes(false)
	pods, _ := util.ParsePods(false)

	Eventually(func(g Gomega) int {
		return util.CountOfStringInSlice("test-daemonset", pods)
	}, "420s", "10s").Should(Equal(len(nodes)),
		"Daemonset pod count does not match node count")
}
