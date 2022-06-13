package restore_test

import (
	"strings"
	"testing"

	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var server1, server2 *testutil.K3sServer
var tmpdDataDir = "/tmp/restoredatadir"
var clientCACertHash string
var testLock int
var restoreServerArgs = []string{"--cluster-init", "-t", "test", "-d", tmpdDataDir}
var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		testLock, err = testutil.K3sTestLock()
		Expect(err).ToNot(HaveOccurred())
		server1, err = testutil.K3sStartServer(restoreServerArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("etcd snapshot restore", func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(restoreServerArgs) {
			Skip("Test needs k3s server with: " + strings.Join(restoreServerArgs, " "))
		}
	})
	When("a snapshot is restored on existing node", func() {
		It("etcd starts up with no problems", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "180s", "5s").Should(Succeed())
		})
		It("create a workload", func() {
			result, err := testutil.K3sCmd("kubectl", "create", "-f", "./testdata/temp_depl.yaml")
			Expect(result).To(ContainSubstring("deployment.apps/nginx-deployment created"))
			Expect(err).NotTo(HaveOccurred())
		})
		It("saves an etcd snapshot", func() {
			Expect(testutil.K3sCmd("etcd-snapshot", "save", "-d", tmpdDataDir, "--name", "snapshot-to-restore")).
				To(ContainSubstring("saved"))
		})
		It("list snapshots", func() {
			Expect(testutil.K3sCmd("etcd-snapshot", "ls", "-d", tmpdDataDir)).
				To(MatchRegexp(`://` + tmpdDataDir + `/server/db/snapshots/snapshot-to-restore`))
		})
		// create another workload
		It("create a workload 2", func() {
			result, err := testutil.K3sCmd("kubectl", "create", "-f", "./testdata/temp_depl2.yaml")
			Expect(result).To(ContainSubstring("deployment.apps/nginx-deployment-post-snapshot created"))
			Expect(err).NotTo(HaveOccurred())
		})
		It("get Client CA cert hash", func() {
			// get md5sum of the CA certs
			var err error
			clientCACertHash, err = testutil.RunCommand("md5sum " + tmpdDataDir + "/server/tls/client-ca.crt | cut -f 1 -d' '")
			Expect(err).ToNot(HaveOccurred())
		})
		It("stop k3s", func() {
			Expect(testutil.K3sKillServer(server1)).To(Succeed())
		})
		It("restore the snapshot", func() {
			// get snapshot file
			filePath, err := testutil.RunCommand(`sudo find ` + tmpdDataDir + `/server -name "*snapshot-to-restore*"`)
			Expect(err).ToNot(HaveOccurred())
			filePath = strings.TrimSuffix(filePath, "\n")
			Eventually(func() (string, error) {
				return testutil.K3sCmd("server", "-d", tmpdDataDir, "--cluster-reset", "--token", "test", "--cluster-reset-restore-path", filePath)
			}, "360s", "5s").Should(ContainSubstring(`restart without --cluster-reset flag now`))
		})
		It("start k3s server", func() {
			var err error
			server2, err = testutil.K3sStartServer(restoreServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("starts up with no problems", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "360s", "5s").Should(Succeed())
		})
		It("Make sure Workload 1 exists", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "deployment", "nginx-deployment")
			}, "360s", "5s").Should(ContainSubstring("3/3"))
		})
		It("Make sure Workload 2 does not exists", func() {
			res, err := testutil.K3sCmd("kubectl", "get", "deployment", "nginx-deployment-post-snapshot")
			Expect(err).To(HaveOccurred())
			Expect(res).To(ContainSubstring("not found"))
		})
		It("check if CA cert hash matches", func() {
			// get md5sum of the CA certs
			var err error
			clientCACertHash2, err := testutil.RunCommand("md5sum " + tmpdDataDir + "/server/tls/client-ca.crt | cut -f 1 -d' '")
			Expect(err).ToNot(HaveOccurred())
			Expect(clientCACertHash2).To(Equal(clientCACertHash))
		})
		It("stop k3s", func() {
			Expect(testutil.K3sKillServer(server2)).To(Succeed())
		})
	})
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		Expect(testutil.K3sKillServer(server1)).To(Succeed())
		Expect(testutil.K3sKillServer(server2)).To(Succeed())
		Expect(testutil.K3sCleanup(testLock, tmpdDataDir)).To(Succeed())
	}
})

func Test_IntegrationEtcdRestoreSnapshot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Etcd Restore Suite")
}
