package integration

import (
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	testutil "github.com/rancher/k3s/tests/util"
)

var secretsEncryptionServer *testutil.K3sServer
var secretsEncryptionDataDir = "/tmp/k3sse"

var secretsEncryptionServerArgs = []string{"--secrets-encryption", "-d", secretsEncryptionDataDir}
var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		Expect(os.MkdirAll(secretsEncryptionDataDir, 0777)).To(Succeed())
		secretsEncryptionServer, err = testutil.K3sStartServer(secretsEncryptionServerArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("secrets encryption rotation", func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() {
			Skip("Test does not support running on existing k3s servers")
		}
	})
	When("A server starts with secrets encryption", func() {
		It("starts up with no problems", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "pods", "-A")
			}, "180s", "1s").Should(MatchRegexp("kube-system.+coredns.+1\\/1.+Running"))
		})
		It("it creates a encryption key", func() {
			result, err := testutil.K3sCmd("secrets-encrypt", "status", "-d", secretsEncryptionDataDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Encryption Status: Enabled"))
			Expect(result).To(ContainSubstring("Current Rotation Stage: start"))
		})
	})
	When("A server rotates encryption keys", func() {
		It("it prepares to rotate", func() {
			Expect(testutil.K3sCmd("secrets-encrypt", "prepare", "-d", secretsEncryptionDataDir)).
				To(ContainSubstring("prepare completed successfully"))

			result, err := testutil.K3sCmd("secrets-encrypt", "status", "-d", secretsEncryptionDataDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Current Rotation Stage: prepare"))
			reg, err := regexp.Compile(`AES-CBC.+aescbckey.*`)
			Expect(err).ToNot(HaveOccurred())
			keys := reg.FindAllString(result, -1)
			Expect(keys).To(HaveLen(2))
			Expect(keys[0]).To(ContainSubstring("aescbckey"))
			Expect(keys[1]).To(ContainSubstring("aescbckey-" + fmt.Sprint(time.Now().Year())))
		})
		It("restarts the server", func() {
			var err error
			Expect(testutil.K3sKillServer(secretsEncryptionServer)).To(Succeed())
			secretsEncryptionServer, err = testutil.K3sStartServer(secretsEncryptionServerArgs...)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "pods", "-A")
			}, "180s", "1s").Should(MatchRegexp("kube-system.+coredns.+1\\/1.+Running"))
		})
		It("rotates the keys", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("secrets-encrypt", "rotate", "-d", secretsEncryptionDataDir)
			}, "10s", "2s").Should(ContainSubstring("rotate completed successfully"))

			result, err := testutil.K3sCmd("secrets-encrypt", "status", "-d", secretsEncryptionDataDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Current Rotation Stage: rotate"))
			reg, err := regexp.Compile(`AES-CBC.+aescbckey.*`)
			Expect(err).ToNot(HaveOccurred())
			keys := reg.FindAllString(result, -1)
			Expect(keys).To(HaveLen(2))
			Expect(keys[0]).To(ContainSubstring("aescbckey-" + fmt.Sprint(time.Now().Year())))
			Expect(keys[1]).To(ContainSubstring("aescbckey"))
		})
		It("restarts the server", func() {
			var err error
			Expect(testutil.K3sKillServer(secretsEncryptionServer)).To(Succeed())
			secretsEncryptionServer, err = testutil.K3sStartServer(secretsEncryptionServerArgs...)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "pods", "-A")
			}, "180s", "1s").Should(MatchRegexp("kube-system.+coredns.+1\\/1.+Running"))
		})
		It("reencrypts the keys", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("secrets-encrypt", "reencrypt", "-d", secretsEncryptionDataDir)
			}, "20s", "5s").Should(ContainSubstring("reencrypt completed successfully"))

			result, err := testutil.K3sCmd("secrets-encrypt", "status", "-d", secretsEncryptionDataDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Current Rotation Stage: reencrypt"))
			reg, err := regexp.Compile(`AES-CBC.+aescbckey.*`)
			Expect(err).ToNot(HaveOccurred())
			keys := reg.FindAllString(result, -1)
			Expect(keys).To(HaveLen(1))
			Expect(keys[0]).To(ContainSubstring("aescbckey-" + fmt.Sprint(time.Now().Year())))
		})
	})
	When("A server disables encryption", func() {
		It("it triggers the disable", func() {
			Expect(testutil.K3sCmd("secrets-encrypt", "disable", "-d", secretsEncryptionDataDir)).
				To(ContainSubstring("secrets-encryption disabled"))

			result, err := testutil.K3sCmd("secrets-encrypt", "status", "-d", secretsEncryptionDataDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Encryption Status: Disabled"))
		})
		It("restarts the server", func() {
			var err error
			Expect(testutil.K3sKillServer(secretsEncryptionServer)).To(Succeed())
			secretsEncryptionServer, err = testutil.K3sStartServer(secretsEncryptionServerArgs...)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "pods", "-A")
			}, "180s", "1s").Should(MatchRegexp("kube-system.+coredns.+1\\/1.+Running"))
		})
		It("reencrypts the keys", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("secrets-encrypt", "reencrypt", "-f", "--skip", "-d", secretsEncryptionDataDir)
			}, "20s", "5s").Should(ContainSubstring("reencrypt completed successfully"))

			result, err := testutil.K3sCmd("secrets-encrypt", "status", "-d", secretsEncryptionDataDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Encryption Status: Disabled"))
		})
	})
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		Expect(testutil.K3sKillServer(secretsEncryptionServer)).To(Succeed())
		Expect(testutil.K3sRemoveDataDir(secretsEncryptionDataDir)).To(Succeed())
	}
})

func Test_IntegrationSecretsEncryption(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Secrets Encryption Suite", []Reporter{
		reporters.NewJUnitReporter("/tmp/results/junit-se.xml"),
	})
}
