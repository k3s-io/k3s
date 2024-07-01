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
var dualStackServeripv6masqArgs = []string{
	"--cluster-init",
	"--cluster-cidr", "10.42.0.0/16,2001:cafe:42::/56",
	"--service-cidr", "10.43.0.0/16,2001:cafe:43::/112",
	"--disable-network-policy",
	"--flannel-ipv6-masq",
}
var testLock int

var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() && os.Getenv("CI") != "true" {
		var err error
		testLock, err = testutil.K3sTestLock()
		Expect(err).ToNot(HaveOccurred())
		server, err = testutil.K3sStartServer(dualStackServeripv6masqArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("flannel-ipv6-masq", Ordered, func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(dualStackServeripv6masqArgs) {
			Skip("Test needs k3s server with: " + strings.Join(dualStackServeripv6masqArgs, " "))
		} else if os.Getenv("CI") == "true" {
			Skip("Github environment does not support IPv6")
		}
	})
	When("a ipv4 and ipv6 cidr is present", func() {
		It("starts up with no problems", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "180s", "10s").Should(Succeed())
		})
		It("creates pods with two IPs", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "exec", "deployment/traefik", "-n", "kube-system", "--", "ip", "a")
			}, "5s", "1s").Should(ContainSubstring("2001:cafe:42:"))
		})
		It("verifies ipv6 masq iptables rule exists", func() {
			Eventually(func() (string, error) {
				return testutil.RunCommand("ip6tables -nt nat -L FLANNEL-POSTRTG")
			}, "5s", "1s").Should(ContainSubstring("MASQUERADE"))
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

func Test_IntegrationFlannelIpv6Masq(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "flannel-ipv6-masq Suite")
}
