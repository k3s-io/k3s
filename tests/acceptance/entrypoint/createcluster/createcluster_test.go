package createcluster

import (
	"fmt"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/core/testcase"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Test:", func() {

	Context("Build Cluster:", func() {

		It("Start Up with no issues", func() {
			testcase.TestBuildCluster(GinkgoT(), false)
		})

		It("Checks Node Status", func() {
			testcase.TestNodeStatus(
				GinkgoT(),
				assert.NodeAssertReadyStatus(),
				nil)
		})

		It("Checks Pod Status", func() {
			testcase.TestPodStatus(
				GinkgoT(),
				assert.PodAssertRestarts(),
				assert.PodAssertReady(),
				assert.PodAssertStatus(),
			)
		})

		It("Verifies ClusterIP Service", func() {
			testcase.TestServiceClusterIp(GinkgoT(), true)
		})

		It("Verifies NodePort Service", func() {
			testcase.TestServiceNodePort(GinkgoT(), true)
		})

		It("Verifies LoadBalancer Service", func() {
			testcase.TestServiceLoadBalancer(GinkgoT(), true)
		})

		It("Verifies Ingress", func() {
			testcase.TestIngress(GinkgoT(), true)
		})

		It("Verifies Daemonset", func() {
			testcase.TestDaemonset(GinkgoT(), true)
		})

		It("Verifies Local Path Provisioner storage", func() {
			testcase.TestLocalPathProvisionerStorage(GinkgoT(), true)
		})

		It("Verifies dns access", func() {
			testcase.TestDnsAccess(GinkgoT(), true)
		})
	})
})

var _ = BeforeEach(func() {
	if *util.Destroy {
		Skip("Cluster is being Deleted")
	}
})

var _ = AfterEach(func() {
	if CurrentSpecReport().Failed() {
		fmt.Printf("\nFAILED! %s\n", CurrentSpecReport().FullText())
	} else {
		fmt.Printf("\nPASSED! %s\n", CurrentSpecReport().FullText())
	}
})
