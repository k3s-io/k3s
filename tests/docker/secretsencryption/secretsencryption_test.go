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
			Expect(tc.ProvisionServers(*serverCount)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(tc.KubeconfigFile)
			}, "60s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})
	Context("Secrets are added without encryption:", func() {
		It("Deploys several secrets", func() {
			_, err := tc.DeployWorkload("secrets.yaml")
			Expect(err).NotTo(HaveOccurred(), "Secrets not deployed")
		})
		It("Verifies encryption disabled", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				res, err := node.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).Should(ContainSubstring("Encryption Status: Disabled, no configuration file found"))
			}
		})
	})
	Context("Secrets encryption is enabled on the cluster:", func() {
		It("Enable secrets-encryption", func() {
			cmd := "k3s secrets-encrypt enable"
			Expect(tc.Servers[0].RunCmdOnNode(cmd)).Error().NotTo(HaveOccurred())
			cmd = "echo 'secrets-encryption: true\n' >> /etc/rancher/k3s/config.yaml"
			for _, node := range tc.Servers {
				Expect(node.RunCmdOnNode(cmd)).Error().NotTo(HaveOccurred())
			}
		})

		It("Restarts K3s servers", func() {
			Expect(docker.RestartCluster(tc.Servers)).To(Succeed())
		})

		It("Verifies encryption start stage", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := node.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("Encryption Status: Disabled"))
					g.Expect(res).Should(ContainSubstring("Current Rotation Stage: start"))
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
				}, "120s", "5s").Should(Succeed())
			}
		})
	})
	Context("Secrets Keys are rotated:", func() {
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
				}, "240s", "10s").Should(Succeed())
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
				}, "240s", "2s").Should(Succeed())
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
				}, "240s", "2s").Should(Succeed())
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
				}, "240s", "2s").Should(Succeed())
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
				}, "240s", "2s").Should(Succeed())
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
				}, "240s", "2s").Should(Succeed())
			}
		})
	})
	Context("Switching to Secretbox Provider", func() {
		It("Append secretbox provider to config", func() {
			cmd := "echo 'secrets-encryption-provider: secretbox' >> /etc/rancher/k3s/config.yaml"
			for _, node := range tc.Servers {
				Expect(node.RunCmdOnNode(cmd)).Error().NotTo(HaveOccurred())
			}
		})
		It("Restarts K3s servers", func() {
			Expect(docker.RestartCluster(tc.Servers)).To(Succeed())
		})

		It("Rotates the Secrets-Encryption Keys, switching to new key type", func() {
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
						g.Expect(res).Should(ContainSubstring("XSalsa20-POLY1305"))
					} else {
						g.Expect(res).Should(ContainSubstring("AES-CBC"))
					}
				}, "240s", "10s").Should(Succeed())
			}
		})

		It("Restarts K3s servers", func() {
			Expect(docker.RestartCluster(tc.Servers)).To(Succeed())
		})

		It("Verifies encryption key matches on all nodes", func() {
			cmd := "k3s secrets-encrypt status"
			for _, node := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := node.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("XSalsa20-POLY1305"))
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
				}, "240s", "2s").Should(Succeed())
			}
		})
	})

})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		log_length := 10
		if *ci {
			log_length = 1000
		}
		AddReportEntry("journald-logs", docker.TailJournalLogs(log_length, append(tc.Servers, tc.Agents...)))
	}
	if *ci || (tc != nil && !failed) {
		Expect(tc.Cleanup()).To(Succeed())
	}
})
