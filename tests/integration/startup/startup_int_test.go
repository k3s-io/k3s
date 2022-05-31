package integration

import (
	"testing"

	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var startupServer *testutil.K3sServer
var startupServerArgs = []string{}
var testLock int

var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		testLock, err = testutil.K3sTestLock()
		Expect(err).ToNot(HaveOccurred())

	}
})

var _ = Describe("startup tests", func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() {
			Skip("Test does not support running on existing k3s servers")
		}
	})
	When("a default server is created", func() {
		It("is created with no arguments", func() {
			var err error
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods deployed", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "60s", "5s").Should(Succeed())
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
		})
	})
	When("a etcd backed server is created", func() {
		It("is created with cluster-init arguments", func() {
			var err error
			startupServerArgs = []string{"--cluster-init"}
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods deployed", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "60s", "5s").Should(Succeed())
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
		})
	})
	When("a server without traefik is created", func() {
		It("is created with disable arguments", func() {
			var err error
			startupServerArgs = []string{"--disable", "traefik"}
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods without traefik deployed", func() {
			Eventually(func() error {
				return testutil.K3sCheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server"})
			}, "60s", "5s").Should(Succeed())
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
		})
	})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		Expect(testutil.K3sCleanup(testLock, "")).To(Succeed())
	}
})

func Test_IntegrationStartup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Startup Suite")
}
