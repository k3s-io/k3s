package integration

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tests "github.com/k3s-io/k3s/tests"
	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	v1 "k8s.io/api/core/v1"
)

var (
	startupServer     *testutil.K3sServer
	startupServerArgs = []string{}
	testLock          int
)

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
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "120s", "5s").Should(Succeed())
		})
		It("has kine without tls", func() {
			Eventually(func() error {
				match, err := testutil.SearchK3sLog(startupServer, "Kine available at unix://kine.sock")
				if err != nil {
					return err
				}
				if match {
					return nil
				}
				return errors.New("error finding kine sock")
			}, "30s", "2s").Should(Succeed())
		})
		It("does not use kine with tls after bootstrap", func() {
			Eventually(func() error {
				match, err := testutil.SearchK3sLog(startupServer, "Kine available at unixs://kine.sock")
				if err != nil {
					return err
				}
				if match {
					return errors.New("Kine with tls when the kine-tls is not set")
				}
				return nil
			}, "30s", "2s").Should(Succeed())
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
		})
	})
	When("a server with kine-tls is created", func() {
		It("is created with kine-tls", func() {
			var err error
			startupServerArgs = []string{"--kine-tls"}
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods deployed", func() {
			Eventually(func() error {
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "120s", "5s").Should(Succeed())
		})
		It("set kine to use tls", func() {
			Eventually(func() error {
				match, err := testutil.SearchK3sLog(startupServer, "Kine available at unixs://kine.sock")
				if err != nil {
					return err
				}
				if match {
					return nil
				}
				return errors.New("error finding unixs://kine.sock")
			}, "30s", "2s").Should(Succeed())
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
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
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
				return tests.CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server"}, testutil.DefaultConfig)
			}, "90s", "10s").Should(Succeed())
			nodes, err := tests.ParseNodes(testutil.DefaultConfig)
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
			Expect(testutil.RunCommand("ip link add dummy4 type dummy")).To(Equal(""))
			Expect(testutil.RunCommand("ip addr add 11.22.33.44/24 dev dummy2")).To(Equal(""))
			Expect(testutil.RunCommand("ip addr add 55.66.77.88/24 dev dummy3")).To(Equal(""))
			Expect(testutil.RunCommand("ip addr add 11.11.22.22/24 dev dummy4")).To(Equal(""))
		})
		It("is created with node-ip arguments", func() {
			var err error
			startupServerArgs = []string{
				"--node-ip", "11.22.33.44",
				"--node-external-ip", "55.66.77.88",
				"--advertise-address", "11.11.22.22",
			}
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the node deployed with correct IPs", func() {
			Eventually(func() error {
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "120s", "10s").Should(Succeed())

			nodes, err := tests.ParseNodes(testutil.DefaultConfig)
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
				},
			}))
		})
		It("get the kubectl and see if has the right advertise ip", func() {
			apiInfo, err := testutil.GetEndpointsAddresses()
			Expect(err).ToNot(HaveOccurred())
			Expect(apiInfo).To(ContainSubstring("11.11.22.22"))
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
			Expect(testutil.RunCommand("ip link del dummy2")).To(Equal(""))
			Expect(testutil.RunCommand("ip link del dummy3")).To(Equal(""))
			Expect(testutil.RunCommand("ip link del dummy4")).To(Equal(""))
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
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "120s", "5s").Should(Succeed())
			nodes, err := tests.ParseNodes(testutil.DefaultConfig)
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
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "120s", "5s").Should(Succeed())
			nodes, err := tests.ParseNodes(testutil.DefaultConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes).To(HaveLen(1))
		})
		var nodes []v1.Node
		It("has a custom node name with id appended", func() {
			var err error
			nodes, err = tests.ParseNodes(testutil.DefaultConfig)
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
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "120s", "5s").Should(Succeed())
			nodes, err := tests.ParseNodes(testutil.DefaultConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodes).To(HaveLen(1))
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
		})
	})
	// Check for regression of containerd restarting pods
	// https://github.com/containerd/containerd/issues/7843
	When("a server with a dummy pod", func() {
		It("is created with no arguments", func() {
			var err error
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods deployed", func() {
			Eventually(func() error {
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "120s", "5s").Should(Succeed())
		})
		It("creates a new pod", func() {
			Expect(testutil.K3sCmd("kubectl apply -f ./testdata/dummy.yaml")).
				To(ContainSubstring("pod/dummy created"))
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl get event -n kube-system --field-selector involvedObject.name=dummy")
			}, "60s", "5s").Should(ContainSubstring("Started container dummy"))
		})
		It("restarts the server", func() {
			var err error
			Expect(testutil.K3sStopServer(startupServer)).To(Succeed())
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "180s", "5s").Should(Succeed())
		})
		It("has the dummy pod not restarted", func() {
			Consistently(func(g Gomega) {
				res, err := testutil.K3sCmd("kubectl get event -n kube-system --field-selector involvedObject.name=dummy")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(res).NotTo(ContainSubstring("Pod sandbox changed, it will be killed and re-created"))
				g.Expect(res).NotTo(ContainSubstring("Stopping container dummy"))
			}, "30s", "5s").Should(Succeed())
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
		})
	})
	When("a server with datastore-endpoint and disable apiserver is created", func() {
		It("is created with datastore-endpoint and disable apiserver flags", func() {
			var err error
			startupServerArgs = []string{"--datastore-endpoint", "test", "--disable-apiserver"}
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).NotTo(HaveOccurred())
		})
		It("search for the error log", func() {
			Eventually(func() error {
				match, err := testutil.SearchK3sLog(startupServer, "invalid flag use; cannot use --disable-apiserver with --datastore-endpoint")
				if err != nil {
					return err
				}
				if match {
					return nil
				}
				return errors.New("nor found error when --datastore-endpoint and --disable-apiserver are used together")
			}, "30s", "2s").Should(Succeed())
		})
		It("cleans up", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
		})
	})
	When("a server with datastore-endpoint and disable etcd is created", func() {
		It("is created with datastore-endpoint and disable etcd flags", func() {
			var err error
			startupServerArgs = []string{"--datastore-endpoint", "test", "--disable-etcd", "-s", "https://192.168.1.12:6443"}
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).NotTo(HaveOccurred())
		})
		It("search for the error log", func() {
			Eventually(func() error {
				match, err := testutil.SearchK3sLog(startupServer, "invalid flag use; cannot use --disable-etcd with --datastore-endpoint")
				if err != nil {
					return err
				}
				if match {
					return nil
				}
				return errors.New("not found error when --datastore-endpoint and --disable-etcd are used together")
			}, "30s", "2s").Should(Succeed())
		})
		It("cleans up", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
		})
	})

})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		if failed {
			testutil.K3sSaveLog(startupServer, false)
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
		}
		Expect(testutil.K3sCleanup(testLock, "")).To(Succeed())
	}
})

func Test_IntegrationStartup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Startup Suite")
}
