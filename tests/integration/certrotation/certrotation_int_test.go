package cert_rotation_test

import (
	"strings"
	"testing"

	tests "github.com/k3s-io/k3s/tests"
	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const tmpdDataDir = "/tmp/certrotationtest"

var server, server2 *testutil.K3sServer
var serverArgs = []string{"--cluster-init", "-t", "test", "-d", tmpdDataDir}
var certHash, caCertHash string
var testLock int

var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		testLock, err = testutil.K3sTestLock()
		Expect(err).ToNot(HaveOccurred())
		server, err = testutil.K3sStartServer(serverArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("certificate rotation", Ordered, func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(serverArgs) {
			Skip("Test needs k3s server with: " + strings.Join(serverArgs, " "))
		}
	})
	When("a new server is created", func() {
		It("starts up with no problems", func() {
			Eventually(func() error {
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "180s", "5s").Should(Succeed())
		})
		It("get certificate hash", func() {
			// get md5sum of the CA certs
			var err error
			caCertHash, err = testutil.RunCommand("md5sum " + tmpdDataDir + "/server/tls/client-ca.crt | cut -f 1 -d' '")
			Expect(err).ToNot(HaveOccurred())
			certHash, err = testutil.RunCommand("md5sum " + tmpdDataDir + "/server/tls/serving-kube-apiserver.crt | cut -f 1 -d' '")
			Expect(err).ToNot(HaveOccurred())
		})
		It("stops k3s", func() {
			Expect(testutil.K3sKillServer(server)).To(Succeed())
		})
		It("rotates certificates", func() {
			_, err := testutil.K3sCmd("certificate", "rotate", "-d", tmpdDataDir)
			Expect(err).ToNot(HaveOccurred())

		})
		It("starts k3s server", func() {
			var err error
			server2, err = testutil.K3sStartServer(serverArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("starts up with no problems", func() {
			Eventually(func() error {
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "360s", "5s").Should(Succeed())
		})
		It("checks the certificate status", func() {
			res, err := testutil.K3sCmd("certificate", "check", "-d", tmpdDataDir)
			Expect(err).ToNot(HaveOccurred())
			for i, line := range strings.Split(res, "\n") {
				// First line is just server info
				if i == 0 || line == "" {
					continue
				}
				Expect(line).To(MatchRegexp("certificate.*is ok|Checking certificates"), res)
			}
		})
		It("gets certificate hash", func() {
			// get md5sum of the CA certs
			var err error
			caCertHashAfter, err := testutil.RunCommand("md5sum " + tmpdDataDir + "/server/tls/client-ca.crt | cut -f 1 -d' '")
			Expect(err).ToNot(HaveOccurred())
			certHashAfter, err := testutil.RunCommand("md5sum " + tmpdDataDir + "/server/tls/serving-kube-apiserver.crt | cut -f 1 -d' '")
			Expect(err).ToNot(HaveOccurred())
			Expect(certHash).To(Not(Equal(certHashAfter)))
			Expect(caCertHash).To(Equal(caCertHashAfter))
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
			testutil.K3sSaveLog(server, false)
		}
		Expect(testutil.K3sKillServer(server)).To(Succeed())
		Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
		Expect(testutil.K3sKillServer(server2)).To(Succeed())
		Expect(testutil.K3sCleanup(testLock, tmpdDataDir)).To(Succeed())
	}
})

func Test_IntegrationCertRotation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cert rotation Suite")
}
