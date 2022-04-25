package integration

import (
	"os/exec"
	"strings"
	"testing"

	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var customEtcdArgsServer *testutil.K3sServer
var customEtcdArgsServerArgs = []string{
	"--cluster-init",
	"--etcd-arg quota-backend-bytes=858993459",
}
var testLock int

var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		testLock, err = testutil.K3sTestLock()
		Expect(err).ToNot(HaveOccurred())
		customEtcdArgsServer, err = testutil.K3sStartServer(customEtcdArgsServerArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("custom etcd args", func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(customEtcdArgsServerArgs) {
			Skip("Test needs k3s server with: " + strings.Join(customEtcdArgsServerArgs, " "))
		}
	})
	When("a custom quota backend bytes is specified", func() {
		It("renders a config file with the correct entry", func() {
			Eventually(func() (string, error) {
				var cmd *exec.Cmd
				grepCmd := "grep"
				grepCmdArgs := []string{"quota-backend-bytes", "/var/lib/rancher/k3s/server/db/etcd/config"}
				if testutil.IsRoot() {
					cmd = exec.Command(grepCmd, grepCmdArgs...)
				} else {
					fullGrepCmd := append([]string{grepCmd}, grepCmdArgs...)
					cmd = exec.Command("sudo", fullGrepCmd...)
				}
				byteOut, err := cmd.CombinedOutput()
				return string(byteOut), err
			}, "45s", "5s").Should(MatchRegexp(".*quota-backend-bytes: 858993459.*"))
		})
	})
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		Expect(testutil.K3sKillServer(customEtcdArgsServer)).To(Succeed())
		Expect(testutil.K3sCleanup(testLock, "")).To(Succeed())
	}
})

func Test_IntegrationCustomEtcdArgs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Custom etcd Arguments")
}
