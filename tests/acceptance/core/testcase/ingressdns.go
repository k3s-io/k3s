package testcase

import (
	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestIngress(deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "ingress.yaml", *util.Arch)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"Ingress manifest not deployed")
	}

	err := assert.ValidateOnHost(util.GetIngressRunning+util.KubeConfigFile, util.RunningAssert)
	if err != nil {
		GinkgoT().Errorf("Error: %v", err)
	}

	ingressIps, err := util.FetchIngressIP()
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Ingress ip is not returned")

	for _, ip := range ingressIps {
		assert.CheckComponentCmdNode("curl -s --header host:foo1.bar.com"+
			" http://"+ip+"/name.html", util.TestIngress, ip)
	}
}

func TestDnsAccess(deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "dnsutils.yaml", *util.Arch)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"dnsutils manifest not deployed")
	}

	err := assert.ValidateOnHost(util.GetPodDnsUtils+util.KubeConfigFile, util.RunningAssert)
	if err != nil {
		GinkgoT().Errorf("Error: %v", err)
	}

	assert.CheckComponentCmdHost(
		util.ExecDnsUtils+util.KubeConfigFile+" -- nslookup kubernetes.default",
		util.Nslookup,
	)
}
