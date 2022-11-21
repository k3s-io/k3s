package integration

import (
	"os"
	"path/filepath"
	"testing"

	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/onsi/gomega/gstruct"
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

var _ = Describe("startup tests", Ordered, func() {

	When("a default server is created", func() {
		It("is created with no arguments", func() {
			var err error
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods deployed", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "120s", "5s").Should(Succeed())
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
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
			}, "120s", "5s").Should(Succeed())
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
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
			nodes, err := testutil.ParseNodes()
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes).To(HaveLen(1))
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
			}, "120s", "10s").Should(Succeed())

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
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
			Expect(testutil.RunCommand("ip link del dummy2")).To(Equal(""))
			Expect(testutil.RunCommand("ip link del dummy3")).To(Equal(""))
		})
	})
	When("a server with different data-dir is created", func() {
		var tempDir string
		It("creates a temp directory", func() {
			var err error
			tempDir, err = os.MkdirTemp("", "k3s-data-dir")
			Expect(err).ToNot(HaveOccurred())
		})
		It("is created with data-dir flag", func() {
			var err error
			startupServerArgs = []string{"--data-dir", tempDir}
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods deployed", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "120s", "5s").Should(Succeed())
			nodes, err := testutil.ParseNodes()
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes).To(HaveLen(1))
		})
		It("has the correct files in the temp data-dir", func() {
			_, err := os.Stat(filepath.Join(tempDir, "server", "tls", "server-ca.key"))
			Expect(err).ToNot(HaveOccurred())
			_, err = os.Stat(filepath.Join(tempDir, "server", "token"))
			Expect(err).ToNot(HaveOccurred())
			_, err = os.Stat(filepath.Join(tempDir, "agent", "client-kubelet.crt"))
			Expect(err).ToNot(HaveOccurred())
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, tempDir)).To(Succeed())
		})
	})
	When("a server with different node options is created", func() {
		It("is created with node-name with-node-id, node-label and node-taint flags", func() {
			var err error
			startupServerArgs = []string{"--node-name", "customnoder", "--with-node-id", "--node-label", "foo=bar", "--node-taint", "alice=bob:PreferNoSchedule"}
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods deployed", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "120s", "5s").Should(Succeed())
			nodes, err := testutil.ParseNodes()
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes).To(HaveLen(1))
		})
		var nodes []v1.Node
		It("has a custom node name with id appended", func() {
			var err error
			nodes, err = testutil.ParseNodes()
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes).To(HaveLen(1))
			Expect(nodes[0].Name).To(MatchRegexp(`-[0-9a-f]*`))
			Expect(nodes[0].Name).To(ContainSubstring("customnoder"))
		})
		It("has proper node labels and taints", func() {
			Expect(nodes[0].ObjectMeta.Labels).To(MatchKeys(IgnoreExtras, Keys{
				"foo": Equal("bar"),
			}))
			Expect(nodes[0].Spec.Taints).To(ContainElement(v1.Taint{Key: "alice", Value: "bob", Effect: v1.TaintEffectPreferNoSchedule}))
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
		})
	})
	When("a server with prefer-bundled-bin option", func() {
		It("is created with prefer-bundled-bin flag", func() {
			var err error
			startupServerArgs = []string{"--prefer-bundled-bin"}
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods deployed", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "120s", "5s").Should(Succeed())
			nodes, err := testutil.ParseNodes()
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes).To(HaveLen(1))
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
		})
	})
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		if CurrentSpecReport().Failed() {
			testutil.K3sDumpLog(startupServer)
		}
		Expect(testutil.K3sCleanup(testLock, "")).To(Succeed())
	}
})

func Test_IntegrationStartup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Startup Suite")
}
