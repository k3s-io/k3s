/*
This test verifies that even if we use flannel-backend=none, kube-api starts correctly so that it can
accept the custom CNI plugin manifest. We want to catch regressions in which kube-api is unresponsive.
To do so we check for 25s that we can consistently query kube-api. We check that pods are in pending
state, which is what should happen if there is no cni plugin
*/
package integration

import (
	"os"
	"strings"
	"testing"

	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var server *testutil.K3sServer
var flannelNoneServerArgs = []string{
	"--flannel-backend=none",
}
var testLock int

var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		testLock, err = testutil.K3sTestLock()
		Expect(err).ToNot(HaveOccurred())
		server, err = testutil.K3sStartServer(flannelNoneServerArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("flannel-backend=none", Ordered, func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(flannelNoneServerArgs) {
			Skip("Test needs k3s server with: " + strings.Join(flannelNoneServerArgs, " "))
		}
	})
	When("Pods can be queried and their status is Pending", func() {
		It("checks pods status", func() {
			// Wait for pods to come up before running the real test
			Eventually(func() int {
				pods, _ := testutil.ParsePodsInNS("kube-system")
				return len(pods)
			}, "180s", "5s").Should(BeNumerically(">", 0))

			pods, err := testutil.ParsePodsInNS("kube-system")
			Expect(err).NotTo(HaveOccurred())

			// Pods should remain in Pending state because there is no network plugin
			Consistently(func() bool {
				for _, pod := range pods {
					if !strings.Contains(string(pod.Status.Phase), "Pending") {
						return false
					}
				}
				return true
			}, "25s").Should(BeTrue())
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() && os.Getenv("CI") != "true" {
		if failed {
			testutil.K3sSaveLog(server, false)
		}
		Expect(testutil.K3sKillServer(server)).To(Succeed())
		Expect(testutil.K3sCleanup(testLock, "")).To(Succeed())
	}
})

func Test_Integrationflannelnone(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "flannel-backend=none Suite")
}
