package secretsencryption

import (
	"flag"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var ci = flag.Bool("ci", false, "running on CI")

func Test_DockerSecretsEncryption(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Secrets Encryption Test Suite", suiteConfig, reporterConfig)
}

var tc *docker.TestConfig

var _ = Describe("Verify Secrets Encryption Rotation", Ordered, func() {
	Context("Setup Cluster", func() {
		It("should provision servers and agents", func() {
			var err error
			tc, err = docker.NewTestConfig("rancher/systemd-node")
			Expect(err).NotTo(HaveOccurred())
			tc.ServerYaml = `secrets-encryption: true`
			Expect(tc.ProvisionServers(*serverCount)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(tc.KubeconfigFile)
			}, "60s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})
	Context("Secrets Keys are rotated:", func() {
		It("Deploys several secrets", func() {
			_, err := tc.DeployWorkload("secrets.yaml")
			Expect(err).NotTo(HaveOccurred(), "Secrets not deployed")
		})

		It("Verifies encryption start stage", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				res, err := node.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
				Expect(res).Should(ContainSubstring("Current Rotation Stage: start"))
				Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
			}
		})

		It("Rotates the Secrets-Encryption Keys", func() {
			cmd := "k3s secrets-encrypt rotate-keys"
			res, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)
			for i, node := range tc.Servers {
				Eventually(func(g Gomega) {
					cmd := "k3s secrets-encrypt status"
					res, err := node.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred(), res)
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: hash does not match"))
					if i == 0 {
						g.Expect(res).Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))
					} else {
						g.Expect(res).Should(ContainSubstring("Current Rotation Stage: start"))
					}
				}, "420s", "10s").Should(Succeed())
			}
		})

		It("Restarts K3s servers", func() {
			Expect(docker.RestartCluster(tc.Servers)).To(Succeed())
		})

		It("Verifies reencryption_finished stage", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := node.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					g.Expect(res).Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
				}, "420s", "2s").Should(Succeed())
			}
		})

	})

	Context("Disabling Secrets-Encryption", func() {
		It("Disables encryption", func() {
			cmd := "k3s secrets-encrypt disable"
			res, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)

			cmd = "k3s secrets-encrypt status"
			Eventually(func() (string, error) {
				return tc.Servers[0].RunCmdOnNode(cmd)
			}, "240s", "10s").Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))

			for i, node := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := node.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred(), res)
					if i == 0 {
						g.Expect(res).Should(ContainSubstring("Encryption Status: Disabled"))
					} else {
						g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					}
				}, "420s", "2s").Should(Succeed())
			}
		})

		It("Restarts K3s servers", func() {
			Expect(docker.RestartCluster(tc.Servers)).To(Succeed())
		})

		It("Verifies encryption disabled on all nodes", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				Eventually(func(g Gomega) {
					g.Expect(node.RunCmdOnNode(cmd)).Should(ContainSubstring("Encryption Status: Disabled"))
				}, "420s", "2s").Should(Succeed())
			}
		})

	})

	Context("Enabling Secrets-Encryption", func() {
		It("Enables encryption", func() {
			cmd := "k3s secrets-encrypt enable"
			res, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)

			cmd = "k3s secrets-encrypt status"
			Eventually(func() (string, error) {
				return tc.Servers[0].RunCmdOnNode(cmd)
			}, "180s", "5s").Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))

			for i, node := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := node.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred(), res)
					if i == 0 {
						g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					} else {
						g.Expect(res).Should(ContainSubstring("Encryption Status: Disabled"))
					}
				}, "420s", "2s").Should(Succeed())
			}
		})

		It("Restarts K3s servers", func() {
			Expect(docker.RestartCluster(tc.Servers)).To(Succeed())
		})

		It("Verifies encryption enabled on all nodes", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				Eventually(func(g Gomega) {
					g.Expect(node.RunCmdOnNode(cmd)).Should(ContainSubstring("Encryption Status: Enabled"))
				}, "420s", "2s").Should(Succeed())
			}
		})
	})

})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if *ci || (tc != nil && !failed) {
		Expect(tc.Cleanup()).To(Succeed())
	}
})
