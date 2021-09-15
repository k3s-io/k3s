package integration

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	testutil "github.com/rancher/k3s/tests/util"
)

var dualStackServerArgs = []string{"--cluster-init", "--cluster-cidr 10.42.0.0/16,2001:cafe:42:0::/56", "--service-cidr 10.43.0.0/16,2001:cafe:42:1::/112"}
var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		server, err = testutil.K3sStartServer(dualStackServerArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("dual stack", func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(dualStackServerArgs) {
			Skip("Test needs k3s server with: " + strings.Join(dualStackServerArgs, " "))
		}
	})
	When("a ipv4 and ipv6 cidr is present", func() {
		It("starts up with no problems", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "pods", "-A")
			}, "90s", "1s").Should(MatchRegexp("kube-system.+traefik.+1\\/1.+Running"))
		})
		It("creates pods with two IPs", func() {
			podname, err := testutil.K3sCmd("kubectl", "get", "pods", "-nkube-system", "-ojsonpath={.items[?(@.metadata.labels.app\\.kubernetes\\.io/name==\"traefik\")].metadata.name}")
			Expect(err).NotTo(HaveOccurred())
			result, err := testutil.K3sCmd("kubectl", "exec", podname, "-nkube-system", "--", "ip", "a")
			Expect(result).To(ContainSubstring("2001:cafe:42:"))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		Expect(testutil.K3sKillServer(server)).To(Succeed())
	}
})

func Test_IntegrationDualStack(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Dual-Stack Suite", []Reporter{
		reporters.NewJUnitReporter("/tmp/results/junit-ls.xml"),
	})
}
