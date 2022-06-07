package integration

import (
	"testing"

	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

var startupServer *testutil.K3sServer
var startupServerArgs = []string{}
var testLock int

var _ = BeforeSuite(func() {
	if testutil.IsExistingServer() {
		Skip("Test does not support running on existing k3s servers")
	}
	var err error
	testLock, err = testutil.K3sTestLock()
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("startup tests", func() {

	When("a default server is created", func() {
		It("is created with no arguments", func() {
			var err error
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods deployed", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "90s", "5s").Should(Succeed())
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
			}, "90s", "5s").Should(Succeed())
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
				return testutil.CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server"})
			}, "90s", "10s").Should(Succeed())
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
		})
	})
	When("a server with different IPs is created", func() {
		It("creates dummy interfaces", func() {
			Expect(testutil.RunCommand("ip link add dummy2 type dummy")).To(Equal(""))
			Expect(testutil.RunCommand("ip link add dummy3 type dummy")).To(Equal(""))
			Expect(testutil.RunCommand("ip addr add 11.22.33.44/24 dev dummy2")).To(Equal(""))
			Expect(testutil.RunCommand("ip addr add 55.66.77.88/24 dev dummy3")).To(Equal(""))
		})
		It("is created with node-ip arguments", func() {
			var err error
			startupServerArgs = []string{"--node-ip", "11.22.33.44", "--node-external-ip", "55.66.77.88"}
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the node deployed with correct IPs", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "90s", "10s").Should(Succeed())

			nodes, err := testutil.ParseNodes()
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes).To(HaveLen(1))
			Expect(nodes[0].Status.Addresses).To(ContainElements([]v1.NodeAddress{
				{
					Type:    "InternalIP",
					Address: "11.22.33.44",
				},
				{
					Type:    "ExternalIP",
					Address: "55.66.77.88",
				}}))
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.RunCommand("ip link del dummy2")).To(Equal(""))
			Expect(testutil.RunCommand("ip link del dummy3")).To(Equal(""))
		})
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
