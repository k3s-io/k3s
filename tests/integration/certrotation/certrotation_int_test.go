package cert_rotation_test

import (
	"strings"
	"testing"

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

var _ = Describe("certificate rotation", func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(serverArgs) {
			Skip("Test needs k3s server with: " + strings.Join(serverArgs, " "))
		}
	})
	When("a new server is created", func() {
		It("starts up with no problems", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
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
		It("stop k3s", func() {
			Expect(testutil.K3sKillServer(server)).To(Succeed())
		})
		It("certificate rotate", func() {
			_, err := testutil.K3sCmd("certificate", "rotate", "-d", tmpdDataDir)
			Expect(err).ToNot(HaveOccurred())

		})
		It("start k3s server", func() {
			var err error
			server2, err = testutil.K3sStartServer(serverArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("starts up with no problems", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "360s", "5s").Should(Succeed())
		})
		It("get certificate hash", func() {
			// get md5sum of the CA certs
			var err error
			caCertHashAfter, err := testutil.RunCommand("md5sum " + tmpdDataDir + "/server/tls/client-ca.crt | cut -f 1 -d' '")
			Expect(err).ToNot(HaveOccurred())
			certHashAfter, err := testutil.RunCommand("md5sum " + tmpdDataDir + "/server/tls/serving-kube-apiserver.crt | cut -f 1 -d' '")
			Expect(err).ToNot(HaveOccurred())
			Expect(caCertHash).To(Not(Equal(certHashAfter)))
			Expect(caCertHash).To(Equal(caCertHashAfter))
		})
	})
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		Expect(testutil.K3sKillServer(server)).To(Succeed())
		Expect(testutil.K3sCleanup(testLock, "")).To(Succeed())
		Expect(testutil.K3sKillServer(server2)).To(Succeed())
		Expect(testutil.K3sCleanup(testLock, tmpdDataDir)).To(Succeed())
	}
})

func Test_IntegrationCertRotation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cert rotation Suite")
}
