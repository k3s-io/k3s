package integration

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	testutil "github.com/rancher/k3s/tests/util"
)

var secretsEncryptionServer *testutil.K3sServer
var secretsEncryptionDataDir = "/tmp/k3sse"
var secretsEncryptionServerArgs = []string{"--secrets-encryption"}

// var secretsEncryptionServerArgs = []string{"--secrets-encryption", "-d", secretsEncryptionDataDir}
var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		err = os.MkdirAll(secretsEncryptionDataDir, 0777)
		Expect(err).ToNot(HaveOccurred())
		secretsEncryptionServer, err = testutil.K3sStartServer(secretsEncryptionServerArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("secrets encryption", func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() {
			Skip("Test does not support running on existing k3s servers")
		}
	})
	When("A server starts with secrets encryption", func() {
		It("starts up with no problems", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "pods", "-A")
			}, "90s", "1s").Should(MatchRegexp("kube-system.+coredns.+1\\/1.+Running"))
		})
		It("it creates a encryption key", func() {
			result, err := testutil.K3sCmd("secrets-encrypt", "status")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Encryption Status: Enabled"))
			Expect(result).To(ContainSubstring("Current Rotation Stage: start"))
		})
	})
	When("A server rotates encryption keys", func() {
		It("it prepares to rotate", func() {
			Expect(testutil.K3sCmd("secrets-encrypt", "prepare")).
				To(ContainSubstring("prepare completed successfully"))

			result, err := testutil.K3sCmd("secrets-encrypt", "status")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Current Rotation Stage: prepare"))
		})
		// It("creates a new pod", func() {
		// 	Expect(testutil.K3sCmd("kubectl", "create", "-f", "../testdata/localstorage_pod.yaml")).
		// 		To(ContainSubstring("pod/volume-test created"))
		// })
		// It("shows storage up in kubectl", func() {
		// 	Eventually(func() (string, error) {
		// 		return testutil.K3sCmd("kubectl", "get", "--namespace=default", "pvc")
		// 	}, "45s", "1s").Should(MatchRegexp(`local-path-pvc.+Bound`))
		// 	Eventually(func() (string, error) {
		// 		return testutil.K3sCmd("kubectl", "get", "--namespace=default", "pv")
		// 	}, "10s", "1s").Should(MatchRegexp(`pvc.+2Gi.+Bound`))
		// 	Eventually(func() (string, error) {
		// 		return testutil.K3sCmd("kubectl", "get", "--namespace=default", "pod")
		// 	}, "10s", "1s").Should(MatchRegexp(`volume-test.+Running`))
		// })
		// It("has proper folder permissions", func() {
		// 	var k3sStorage = "/var/lib/rancher/k3s/storage"
		// 	fileStat, err := os.Stat(k3sStorage)
		// 	Expect(err).ToNot(HaveOccurred())
		// 	Expect(fmt.Sprintf("%04o", fileStat.Mode().Perm())).To(Equal("0701"))

		// 	pvResult, err := testutil.K3sCmd("kubectl", "get", "--namespace=default", "pv")
		// 	Expect(err).ToNot(HaveOccurred())
		// 	reg, err := regexp.Compile(`pvc[^\s]+`)
		// 	Expect(err).ToNot(HaveOccurred())
		// 	volumeName := reg.FindString(pvResult) + "_default_local-path-pvc"
		// 	fileStat, err = os.Stat(k3sStorage + "/" + volumeName)
		// 	Expect(err).ToNot(HaveOccurred())
		// 	Expect(fmt.Sprintf("%04o", fileStat.Mode().Perm())).To(Equal("0777"))
		// })
		// It("deletes properly", func() {
		// 	Expect(testutil.K3sCmd("kubectl", "delete", "--namespace=default", "--force", "pod", "volume-test")).
		// 		To(ContainSubstring("pod \"volume-test\" force deleted"))
		// 	Expect(testutil.K3sCmd("kubectl", "delete", "--namespace=default", "pvc", "local-path-pvc")).
		// 		To(ContainSubstring("persistentvolumeclaim \"local-path-pvc\" deleted"))
		// })
	})
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		Expect(testutil.K3sKillServer(secretsEncryptionServer)).To(Succeed())
		os.RemoveAll(secretsEncryptionDataDir)
	}
})

func Test_IntegrationSecretsEncryption(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Secrets Encryption Suite", []Reporter{
		reporters.NewJUnitReporter("/tmp/results/junit-se.xml"),
	})
}
