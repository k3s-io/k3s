package ca_cert_rotation_test

import (
	"fmt"
	"strings"
	"testing"

	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const tmpdDataDir = "/tmp/cacertrotationtest"

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

var _ = Describe("ca certificate rotation", Ordered, func() {
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
		It("generates updated ca-certificates", func() {
			cmd := fmt.Sprintf("DATA_DIR=%s ../../../contrib/util/rotate-default-ca-certs.sh", tmpdDataDir)
			By("running command: " + cmd)
			res, err := testutil.RunCommand(cmd)
			By("checking command results: " + res)
			Expect(err).ToNot(HaveOccurred())
		})
		It("certificate rotate-ca", func() {
			res, err := testutil.K3sCmd("certificate", "rotate-ca", "-d", tmpdDataDir, "--path", tmpdDataDir+"/server/rotate-ca")
			By("checking command results: " + res)
			Expect(err).ToNot(HaveOccurred())
		})
		It("stop k3s", func() {
			Expect(testutil.K3sKillServer(server)).To(Succeed())
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
			Expect(certHash).To(Not(Equal(certHashAfter)))
			Expect(caCertHash).To(Not(Equal(caCertHashAfter)))
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
	RunSpecs(t, "CA Cert rotation Suite")
}
