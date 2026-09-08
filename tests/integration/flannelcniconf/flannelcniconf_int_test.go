/*
This test verifies that the file passed as --flannel-cni-conf is what the embedded flannel
installs as the CNI conflist, rather than the built-in template. The custom file is the
built-in template with an mtu on the bridge delegate, a typical use of the flag, so the
node must also still come up with it.
*/
package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tests "github.com/k3s-io/k3s/tests"
	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The conflist the embedded flannel writes, under the default data dir the harness cleans up.
const installedCNIConf = "/var/lib/rancher/k3s/agent/etc/cni/net.d/10-flannel.conflist"

var server *testutil.K3sServer
var flannelCNIConfServerArgs = []string{"--flannel-cni-conf"}
var customCNIConfFile string
var testLock int

var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		customCNIConfFile, err = filepath.Abs("./testdata/custom.conflist")
		Expect(err).ToNot(HaveOccurred())
		flannelCNIConfServerArgs = append(flannelCNIConfServerArgs, customCNIConfFile)
		testLock, err = testutil.K3sTestLock()
		Expect(err).ToNot(HaveOccurred())
		server, err = testutil.K3sStartServer(flannelCNIConfServerArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("flannel-cni-conf", Ordered, func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() {
			if !testutil.ServerArgsPresent(flannelCNIConfServerArgs) {
				Skip("Test needs k3s server with: " + strings.Join(flannelCNIConfServerArgs, " "))
			}
			// The file is whichever one the existing server was started with.
			args := testutil.K3sServerArgs()
			for i, arg := range args {
				if arg == "--flannel-cni-conf" && i+1 < len(args) {
					customCNIConfFile = args[i+1]
				}
			}
		}
	})
	When("a custom CNI conflist is passed", func() {
		It("installs the custom conflist rather than the built-in template", func() {
			expected, err := os.ReadFile(customCNIConfFile)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() (string, error) {
				content, err := os.ReadFile(installedCNIConf)
				return string(content), err
			}, "120s", "5s").Should(Equal(string(expected)))
		})
		It("still starts the default deployments with it", func() {
			Eventually(func() error {
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "180s", "10s").Should(Succeed())
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
			testutil.K3sCopyPodLogs(server)
			testutil.K3sDumpResources(server, "node", "pod", "pvc", "pv")
		}
		Expect(testutil.K3sKillServer(server)).To(Succeed())
		Expect(testutil.K3sCleanup(testLock, "")).To(Succeed())
	}
})

func Test_IntegrationFlannelCNIConf(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "flannel-cni-conf Suite")
}
