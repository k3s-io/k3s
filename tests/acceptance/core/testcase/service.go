package testcase

import (
	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	util2 "github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestServiceClusterIp(g ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util2.ManageWorkload("create", "clusterip.yaml", *util2.Arch)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"Cluster IP manifest not deployed")
	}

	err := assert.ValidateOnHost(util2.GetClusterIp+util2.KubeConfigFile, util2.RunningAssert)
	if err != nil {
		ginkgo.GinkgoT().Logf("Error: %v", err)
	}

	clusterip, _ := util2.FetchClusterIP(util2.NginxClusterIpSVC)
	nodeExternalIP := util2.FetchNodeExternalIP()
	for _, ip := range nodeExternalIP {
		err = assert.ValidateOnNode(ip, "curl -sL --insecure http://"+clusterip+"/name.html",
			util2.TestClusterip)
		if err != nil {
			ginkgo.GinkgoT().Logf("Error: %v", err)
		}
	}
}

func TestServiceNodePort(g ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util2.ManageWorkload("create", "nodeport.yaml", *util2.Arch)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"NodePort manifest not deployed")
	}

	nodeExternalIP := util2.FetchNodeExternalIP()
	nodeport, err := util2.FetchServiceNodePort(util2.NginxNodePortSVC)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	for _, ip := range nodeExternalIP {
		err = assert.ValidateOnHost(
			util2.GetNodeport+util2.KubeConfigFile,
			util2.RunningAssert,
			"curl -sL --insecure http://"+""+ip+":"+nodeport+"/name.html",
			util2.TestNodePort,
		)
		if err != nil {
			ginkgo.GinkgoT().Logf("Error: %v", err)
		}
	}
}

func TestServiceLoadBalancer(g ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util2.ManageWorkload("create", "loadbalancer.yaml", *util2.Arch)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"Loadbalancer manifest not deployed")
	}

	port, err := util2.RunCommandHost(util2.GetLoadbalancerSVC + util2.KubeConfigFile)
	if err != nil {
		ginkgo.GinkgoT().Logf("Error: %v", err)
	}

	nodeExternalIP := util2.FetchNodeExternalIP()
	for _, ip := range nodeExternalIP {
		err = assert.ValidateOnHost(
			util2.GetAppLoadBalancer+util2.KubeConfigFile,
			util2.TestLoadBalancer,
			"curl -sL --insecure http://"+ip+":"+port+"/name.html",
			util2.TestLoadBalancer,
		)
		if err != nil {
			ginkgo.GinkgoT().Logf("Error: %v", err)
		}
	}
}
