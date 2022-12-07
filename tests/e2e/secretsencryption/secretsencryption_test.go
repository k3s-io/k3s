package secretsencryption

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS: generic/ubuntu2004, opensuse/Leap-15.3.x86_64, dweomer/microos.amd64
var nodeOS = flag.String("nodeOS", "generic/ubuntu2004", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var hardened = flag.Bool("hardened", false, "true or false")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 or nil for latest commit from master

func Test_E2ESecretsEncryption(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Secrets Encryption Test Suite", suiteConfig, reporterConfig)
}

var (
	kubeConfigFile  string
	serverNodeNames []string
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Secrets Encryption Rotation", Ordered, func() {
	Context("Cluster :", func() {
		It("Starts up with no issues", func() {
			var err error
			serverNodeNames, _, err = e2e.CreateCluster(*nodeOS, *serverCount, 0)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog)
			fmt.Println("CLUSTER CONFIG")
			fmt.Println("OS:", *nodeOS)
			fmt.Println("Server Nodes:", serverNodeNames)
			kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(kubeConfigFile, true)

			fmt.Printf("\nFetching pods status\n")
			Eventually(func(g Gomega) {
				pods, err := e2e.ParsePods(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
					} else {
						g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
					}
				}
			}, "420s", "5s").Should(Succeed())
			_, _ = e2e.ParsePods(kubeConfigFile, true)
		})

		It("Deploys several secrets", func() {
			_, err := e2e.DeployWorkload("secrets.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "Secrets not deployed")
		})

		It("Verifies encryption start stage", func() {
			cmd := "sudo k3s secrets-encrypt status"
			for _, nodeName := range serverNodeNames {
				res, err := e2e.RunCmdOnNode(cmd, nodeName)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
				Expect(res).Should(ContainSubstring("Current Rotation Stage: start"))
				Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
			}
		})

		It("Prepares for Secrets-Encryption Rotation", func() {
			cmd := "sudo k3s secrets-encrypt prepare"
			res, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), res)
			for i, nodeName := range serverNodeNames {
				cmd := "sudo k3s secrets-encrypt status"
				res, err := e2e.RunCmdOnNode(cmd, nodeName)
				Expect(err).NotTo(HaveOccurred(), res)
				Expect(res).Should(ContainSubstring("Server Encryption Hashes: hash does not match"))
				if i == 0 {
					Expect(res).Should(ContainSubstring("Current Rotation Stage: prepare"))
				} else {
					Expect(res).Should(ContainSubstring("Current Rotation Stage: start"))
				}
			}
		})

		It("Restarts K3s servers", func() {
			Expect(e2e.RestartCluster(serverNodeNames)).To(Succeed())
		})

		It("Checks node and pod status", func() {
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())

			Eventually(func(g Gomega) {
				pods, err := e2e.ParsePods(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
					} else {
						g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
					}
				}
			}, "420s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(kubeConfigFile, true)
		})

		It("Verifies encryption prepare stage", func() {
			cmd := "sudo k3s secrets-encrypt status"
			for _, nodeName := range serverNodeNames {
				Eventually(func(g Gomega) {
					res, err := e2e.RunCmdOnNode(cmd, nodeName)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					g.Expect(res).Should(ContainSubstring("Current Rotation Stage: prepare"))
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
				}, "420s", "2s").Should(Succeed())
			}
		})

		It("Rotates the Secrets-Encryption Keys", func() {
			cmd := "sudo k3s secrets-encrypt rotate"
			res, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), res)
			for i, nodeName := range serverNodeNames {
				Eventually(func(g Gomega) {
					cmd := "sudo k3s secrets-encrypt status"
					res, err := e2e.RunCmdOnNode(cmd, nodeName)
					g.Expect(err).NotTo(HaveOccurred(), res)
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: hash does not match"))
					if i == 0 {
						g.Expect(res).Should(ContainSubstring("Current Rotation Stage: rotate"))
					} else {
						g.Expect(res).Should(ContainSubstring("Current Rotation Stage: prepare"))
					}
				}, "420s", "2s").Should(Succeed())
			}
		})

		It("Restarts K3s servers", func() {
			Expect(e2e.RestartCluster(serverNodeNames)).To(Succeed())
		})

		It("Verifies encryption rotate stage", func() {
			cmd := "sudo k3s secrets-encrypt status"
			for _, nodeName := range serverNodeNames {
				Eventually(func(g Gomega) {
					res, err := e2e.RunCmdOnNode(cmd, nodeName)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					g.Expect(res).Should(ContainSubstring("Current Rotation Stage: rotate"))
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
				}, "420s", "2s").Should(Succeed())
			}
		})

		It("Reencrypts the Secrets-Encryption Keys", func() {
			cmd := "sudo k3s secrets-encrypt reencrypt"
			res, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), res)

			cmd = "sudo k3s secrets-encrypt status"
			Eventually(func() (string, error) {
				return e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			}, "180s", "5s").Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))

			for _, nodeName := range serverNodeNames[1:] {
				res, err := e2e.RunCmdOnNode(cmd, nodeName)
				Expect(err).NotTo(HaveOccurred(), res)
				Expect(res).Should(ContainSubstring("Server Encryption Hashes: hash does not match"))
				Expect(res).Should(ContainSubstring("Current Rotation Stage: rotate"))
			}
		})

		It("Restarts K3s Servers", func() {
			Expect(e2e.RestartCluster(serverNodeNames)).To(Succeed())
		})

		It("Verifies Encryption Reencrypt Stage", func() {
			cmd := "sudo k3s secrets-encrypt status"
			for _, nodeName := range serverNodeNames {
				Eventually(func(g Gomega) {
					res, err := e2e.RunCmdOnNode(cmd, nodeName)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					g.Expect(res).Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
				}, "420s", "2s").Should(Succeed())
			}
		})
	})

	It("Disables encryption", func() {
		cmd := "sudo k3s secrets-encrypt disable"
		res, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred(), res)

		cmd = "sudo k3s secrets-encrypt reencrypt -f --skip"
		res, err = e2e.RunCmdOnNode(cmd, serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred(), res)

		cmd = "sudo k3s secrets-encrypt status"
		Eventually(func() (string, error) {
			return e2e.RunCmdOnNode(cmd, serverNodeNames[0])
		}, "180s", "5s").Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))

		for i, nodeName := range serverNodeNames {
			Eventually(func(g Gomega) {
				res, err := e2e.RunCmdOnNode(cmd, nodeName)
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
		Expect(e2e.RestartCluster(serverNodeNames)).To(Succeed())
	})

	It("Verifies encryption disabled on all nodes", func() {
		cmd := "sudo k3s secrets-encrypt status"
		for _, nodeName := range serverNodeNames {
			Eventually(func(g Gomega) {
				g.Expect(e2e.RunCmdOnNode(cmd, nodeName)).Should(ContainSubstring("Encryption Status: Disabled"))
			}, "420s", "2s").Should(Succeed())
		}
	})

	It("Enables encryption", func() {
		cmd := "sudo k3s secrets-encrypt enable"
		res, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred(), res)

		cmd = "sudo k3s secrets-encrypt reencrypt -f --skip"
		res, err = e2e.RunCmdOnNode(cmd, serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred(), res)

		cmd = "sudo k3s secrets-encrypt status"
		Eventually(func() (string, error) {
			return e2e.RunCmdOnNode(cmd, serverNodeNames[0])
		}, "180s", "5s").Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))
	})

	It("Restarts K3s servers", func() {
		Expect(e2e.RestartCluster(serverNodeNames)).To(Succeed())
	})

	It("Verifies encryption enabled on all nodes", func() {
		cmd := "sudo k3s secrets-encrypt status"
		for _, nodeName := range serverNodeNames {
			Eventually(func(g Gomega) {
				g.Expect(e2e.RunCmdOnNode(cmd, nodeName)).Should(ContainSubstring("Encryption Status: Enabled"))
			}, "420s", "2s").Should(Succeed())
		}

	})

})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		fmt.Println("FAILED!")
	} else {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
