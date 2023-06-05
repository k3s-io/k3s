package createcluster

import (
	"fmt"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/core/testcase"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Test:", func() {

	It("Start Up with no issues", func() {
		testcase.TestBuildCluster(GinkgoT())
	})

	It("Validate Node", func() {
		testcase.TestNodeStatus(
			assert.NodeAssertReadyStatus(),
			nil,
		)
	})

	It("Validate Pod", func() {
		testcase.TestPodStatus(
			assert.PodAssertRestart(),
			assert.PodAssertReady(),
			assert.PodAssertStatus(),
		)
	})

	It("Verifies ClusterIP Service", func() {
		testcase.TestServiceClusterIp(true)
	})

	It("Verifies NodePort Service", func() {
		testcase.TestServiceNodePort(true)
	})

	It("Verifies LoadBalancer Service", func() {
		testcase.TestServiceLoadBalancer(true)
	})

	It("Verifies Ingress", func() {
		testcase.TestIngress(true)
	})

	It("Verifies Daemonset", func() {
		testcase.TestDaemonset(true)
	})

	It("Verifies Local Path Provisioner storage", func() {
		testcase.TestLocalPathProvisionerStorage(true)
	})

	It("Verifies dns access", func() {
		testcase.TestDnsAccess(true)
	})
})

var _ = AfterEach(func() {
	if CurrentSpecReport().Failed() {
		fmt.Printf("\nFAILED! %s\n", CurrentSpecReport().FullText())
	} else {
		fmt.Printf("\nPASSED! %s\n", CurrentSpecReport().FullText())
	}
})
