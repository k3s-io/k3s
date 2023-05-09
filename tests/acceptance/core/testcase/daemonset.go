package testcase

import (
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestDaemonset(g ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "daemonset.yaml", *util.Arch)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"Daemonset manifest not deployed")
	}
	nodes, _ := util.ParseNodes(false)
	pods, _ := util.ParsePods(false)

	gomega.Eventually(func(g gomega.Gomega) {
		count := util.CountOfStringInSlice(util.TestDaemonset, pods)
		g.Expect(len(nodes)).Should(gomega.Equal(count),
			"Daemonset pod count does not match node count")
	}, "420s", "10s").Should(gomega.Succeed())
}
