package testcase

import (
	util2 "github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestDaemonset(g ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util2.ManageWorkload("create", "daemonset.yaml", *util2.Arch)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"Daemonset manifest not deployed")
	}
	nodes, _ := util2.ParseNodes(false)
	pods, _ := util2.ParsePods(false)

	gomega.Eventually(func(g gomega.Gomega) {
		count := util2.CountOfStringInSlice(util2.TestDaemonset, pods)
		g.Expect(len(nodes)).Should(gomega.Equal(count),
			"Daemonset pod count does not match node count")
	}, "420s", "10s").Should(gomega.Succeed())
}
