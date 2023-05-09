package testcase

import (
	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestServiceClusterIp(g ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "clusterip.yaml", *util.Arch)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"Cluster IP manifest not deployed")
	}

	err := assert.ValidateOnHost(util.GetClusterIp+util.KubeConfigFile, util.RunningAssert)
	if err != nil {
		ginkgo.GinkgoT().Logf("Error: %v", err)
	}

	clusterip, _ := util.FetchClusterIP(util.NginxClusterIpSVC)
	nodeExternalIP := util.FetchNodeExternalIP()
	for _, ip := range nodeExternalIP {
		err = assert.ValidateOnNode(ip, "curl -sL --insecure http://"+clusterip+"/name.html",
			util.TestClusterip)
		if err != nil {
			ginkgo.GinkgoT().Logf("Error: %v", err)
		}
	}
}

func TestServiceNodePort(g ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "nodeport.yaml", *util.Arch)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"NodePort manifest not deployed")
	}

	nodeExternalIP := util.FetchNodeExternalIP()
	nodeport, err := util.FetchServiceNodePort(util.NginxNodePortSVC)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	for _, ip := range nodeExternalIP {
		err = assert.ValidateOnHost(
			util.GetNodeport+util.KubeConfigFile,
			util.RunningAssert,
			"curl -sL --insecure http://"+""+ip+":"+nodeport+"/name.html",
			util.TestNodePort,
		)
		if err != nil {
			ginkgo.GinkgoT().Logf("Error: %v", err)
		}
	}
}

func TestServiceLoadBalancer(g ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "loadbalancer.yaml", *util.Arch)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"Loadbalancer manifest not deployed")
	}

	port, err := util.RunCommandHost(util.GetLoadbalancerSVC + util.KubeConfigFile)
	if err != nil {
		ginkgo.GinkgoT().Logf("Error: %v", err)
	}

	nodeExternalIP := util.FetchNodeExternalIP()
	for _, ip := range nodeExternalIP {
		err = assert.ValidateOnHost(
			util.GetAppLoadBalancer+util.KubeConfigFile,
			util.TestLoadBalancer,
			"curl -sL --insecure http://"+ip+":"+port+"/name.html",
			util.TestLoadBalancer,
		)
		if err != nil {
			ginkgo.GinkgoT().Logf("Error: %v", err)
		}
	}
}
