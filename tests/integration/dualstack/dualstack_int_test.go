package integration

import (
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	testutil "github.com/rancher/k3s/tests/util"
)

var dualStackServer *testutil.K3sServer
var dualStackServerArgs = []string{
	"--cluster-init",
	"--cluster-cidr 10.42.0.0/16,2001:cafe:42:0::/56",
	"--service-cidr 10.43.0.0/16,2001:cafe:42:1::/112",
	"--disable-network-policy",
}
var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() && os.Getenv("CI") != "true" {
		var err error
		dualStackServer, err = testutil.K3sStartServer(dualStackServerArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("dual stack", func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(dualStackServerArgs) {
			Skip("Test needs k3s server with: " + strings.Join(dualStackServerArgs, " "))
		} else if os.Getenv("CI") == "true" {
			Skip("Github environment does not support IPv6")
		}
	})
	When("a ipv4 and ipv6 cidr is present", func() {
		It("starts up with no problems", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "pods", "-A")
			}, "180s", "5s").Should(MatchRegexp("kube-system.+traefik.+1\\/1.+Running"))
		})
		It("creates pods with two IPs", func() {
			podname, err := testutil.K3sCmd("kubectl", "get", "pods", "-n", "kube-system", "-o", "jsonpath={.items[?(@.metadata.labels.app\\.kubernetes\\.io/name==\"traefik\")].metadata.name}")
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "exec", podname, "-n", "kube-system", "--", "ip", "a")
			}, "5s", "1s").Should(ContainSubstring("2001:cafe:42:"))
		})
	})
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() && os.Getenv("CI") != "true" {
		Expect(testutil.K3sKillServer(dualStackServer, false)).To(Succeed())
		Expect(testutil.K3sCleanup(dualStackServer, true, "")).To(Succeed())
	}
})

func Test_IntegrationDualStack(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Dual-Stack Suite", []Reporter{
		reporters.NewJUnitReporter("/tmp/results/junit-ls.xml"),
	})
}
