package testcase

import (
	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	util2 "github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestIngress(g ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util2.ManageWorkload("create", "ingress.yaml", *util2.Arch)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"Ingress manifest not deployed")
	}

	err := assert.ValidateOnHost(util2.GetIngressRunning+util2.KubeConfigFile, util2.RunningAssert)
	if err != nil {
		ginkgo.GinkgoT().Logf("Error: %v", err)
	}

	ingressIps, err := util2.FetchIngressIP()
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Ingress ip is not returned")

	for _, ip := range ingressIps {
		_ = assert.CheckComponentCmdHost([]string{"curl -s --header host:foo1.bar.com" +
			" http://" + ip + "/name.html"}, util2.TestIngress)
	}
}

func TestDnsAccess(g ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util2.ManageWorkload("create", "dnsutils.yaml", *util2.Arch)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"dnsutils manifest not deployed")
	}

	exec := "kubectl exec -t dnsutils --kubeconfig=" +
		util2.KubeConfigFile + " -- nslookup kubernetes.default"
	err := assert.ValidateOnHost(
		util2.GetPodDnsUtils+util2.KubeConfigFile,
		util2.RunningAssert,
		exec,
		util2.Nslookup,
	)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}
