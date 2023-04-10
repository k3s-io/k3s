package upgradecluster

import (
	"fmt"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/core/testcase"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Test:", func() {

	Context("Build cluster:", func() {

		It("Start Up with no issues", func() {
			testcase.TestBuildCluster(GinkgoT(), false)
		})

		It("Checks Node Status", func() {
			testcase.TestNodeStatus(
				GinkgoT(),
				assert.NodeAssertReadyStatus(),
				nil,
			)
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
	Context("Upgrade:", func() {

		It("Verify Cluster is upgraded", func() {
			testcase.TestUpgradeCluster(GinkgoT())
		})

		It("Checks Node Status pos upgrade and validate version", func() {
			testcase.TestNodeStatus(
				GinkgoT(),
				assert.NodeAssertReadyStatus(),
				assert.NodeAssertVersionUpgraded(),
			)
		})

		It("Checks Pod Status pos upgrade", func() {
			testcase.TestPodStatus(
				GinkgoT(),
				assert.PodAssertRestarts(),
				assert.PodAssertReady(),
				assert.PodAssertStatus(),
			)
		})

		It("Verifies ClusterIP Service after upgrade", func() {
			testcase.TestServiceClusterIp(GinkgoT(), false)
		})

		It("Verifies NodePort Service after upgrade", func() {
			testcase.TestServiceNodePort(GinkgoT(), false)
		})

		It("Verifies Ingress after upgrade", func() {
			testcase.TestIngress(GinkgoT(), false)
		})

		It("Verifies Daemonset after upgrade", func() {
			testcase.TestDaemonset(GinkgoT(), false)
		})

		It("Verifies LoadBalancer Service after upgrade", func() {
			testcase.TestServiceLoadBalancer(GinkgoT(), false)
		})

		It("Verifies Local Path Provisioner storage after upgrade", func() {
			testcase.TestLocalPathProvisionerStorage(GinkgoT(), false)
		})

		It("Verifies dns access after upgrade", func() {
			testcase.TestDnsAccess(GinkgoT(), false)
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
