package testcase

import (
	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIngress(deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "ingress.yaml", *util.Arch)
		Expect(err).NotTo(HaveOccurred(),
			"Ingress manifest not deployed")
	}

	getIngressRunning := "kubectl get pods  -l k8s-app=nginx-app-ingress" +
		" --field-selector=status.phase=Running  --kubeconfig="
	err := assert.ValidateOnHost(getIngressRunning+util.KubeConfigFile, util.Running)
	if err != nil {
		GinkgoT().Errorf("%v", err)
	}

	ingressIps, err := util.FetchIngressIP()
	Expect(err).NotTo(HaveOccurred(), "Ingress ip is not returned")

	for _, ip := range ingressIps {
		assert.CheckComponentCmdNode("curl -s --header host:foo1.bar.com"+
			" http://"+ip+"/name.html", "test-ingress", ip)
	}
}

func TestDnsAccess(deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "dnsutils.yaml", *util.Arch)
		Expect(err).NotTo(HaveOccurred(),
			"dnsutils manifest not deployed")
	}

	getPodDnsUtils := "kubectl get pods dnsutils --kubeconfig="
	err := assert.ValidateOnHost(getPodDnsUtils+util.KubeConfigFile, util.Running)
	if err != nil {
		GinkgoT().Errorf("%v", err)
	}

	execDnsUtils := "kubectl exec -t dnsutils --kubeconfig="
	assert.CheckComponentCmdHost(
		execDnsUtils+util.KubeConfigFile+" -- nslookup kubernetes.default",
		"kubernetes.default.svc.cluster.local",
	)
}
