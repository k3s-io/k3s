package startup

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 or nil for latest commit from master

// This test suite is used to verify that K3s can start up with dynamic configurations that require
// both server and agent nodes. It is unique in passing dynamic arguments to vagrant, unlike the
// rest of the E2E tests, which use static Vagrantfiles and cluster configurations.
// If you have a server only flag, the startup integration test is a better place to test it.

func Test_E2EStartupValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Startup Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

func StartK3sCluster(nodes []e2e.VagrantNode, serverYAML string, agentYAML string) error {

	for _, node := range nodes {
		var yamlCmd string
		var resetCmd string
		var startCmd string
		if strings.Contains(node.String(), "server") {
			resetCmd = "head -n 3 /etc/rancher/k3s/config.yaml > /tmp/config.yaml && sudo mv /tmp/config.yaml /etc/rancher/k3s/config.yaml"
			yamlCmd = fmt.Sprintf("echo '%s' >> /etc/rancher/k3s/config.yaml", serverYAML)
			startCmd = "systemctl start k3s"
		} else {
			resetCmd = "head -n 4 /etc/rancher/k3s/config.yaml > /tmp/config.yaml && sudo mv /tmp/config.yaml /etc/rancher/k3s/config.yaml"
			yamlCmd = fmt.Sprintf("echo '%s' >> /etc/rancher/k3s/config.yaml", agentYAML)
			startCmd = "systemctl start k3s-agent"
		}
		if _, err := node.RunCmdOnNode(resetCmd); err != nil {
			return err
		}
		if _, err := node.RunCmdOnNode(yamlCmd); err != nil {
			return err
		}
		if _, err := node.RunCmdOnNode(startCmd); err != nil {
			return &e2e.NodeError{Node: node, Cmd: startCmd, Err: err}
		}
	}
	return nil
}

func KillK3sCluster(nodes []e2e.VagrantNode) error {
	for _, node := range nodes {
		if _, err := node.RunCmdOnNode("k3s-killall.sh"); err != nil {
			return err
		}
		if _, err := node.RunCmdOnNode("journalctl --flush --sync --rotate --vacuum-size=1"); err != nil {
			return err
		}
		if _, err := node.RunCmdOnNode("rm -rf /etc/rancher/k3s/config.yaml.d"); err != nil {
			return err
		}
		if strings.Contains(node.String(), "server") {
			if _, err := node.RunCmdOnNode("rm -rf /var/lib/rancher/k3s/server/db"); err != nil {
				return err
			}
		}
	}
	return nil
}

var _ = ReportAfterEach(e2e.GenReport)

var _ = BeforeSuite(func() {
	var err error
	if *local {
		tc, err = e2e.CreateLocalCluster(*nodeOS, 1, 1)
	} else {
		tc, err = e2e.CreateCluster(*nodeOS, 1, 1)
	}
	Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
})

var _ = Describe("Various Startup Configurations", Ordered, func() {
	Context("Verify dedicated supervisor port", func() {
		It("Starts K3s with no issues", func() {
			for _, node := range tc.Agents {
				cmd := "mkdir -p /etc/rancher/k3s/config.yaml.d; grep -F server: /etc/rancher/k3s/config.yaml | sed s/6443/9345/ > /tmp/99-server.yaml; sudo mv /tmp/99-server.yaml /etc/rancher/k3s/config.yaml.d/"
				res, err := node.RunCmdOnNode(cmd)
				By("checking command results: " + res)
				Expect(err).NotTo(HaveOccurred())
			}
			supervisorPortYAML := "supervisor-port: 9345\napiserver-port: 6443\napiserver-bind-address: 0.0.0.0\ndisable: traefik\nnode-taint: node-role.kubernetes.io/control-plane:NoExecute"
			err := StartK3sCluster(append(tc.Servers, tc.Agents...), supervisorPortYAML, "")
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))

			By("CLUSTER CONFIG")
			By("OS:" + *nodeOS)
			By(tc.Status())
			tc.KubeconfigFile, err = e2e.GenKubeconfigFile(tc.Servers[0].String())
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status", func() {
			By("Fetching node status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "360s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "360s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server"}, tc.KubeconfigFile)
			}, "300s", "10s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})

		It("Returns pod metrics", func() {
			cmd := "kubectl top pod -A"
			Eventually(func() error {
				_, err := e2e.RunCommand(cmd)
				return err
			}, "600s", "5s").Should(Succeed())
		})

		It("Returns node metrics", func() {
			cmd := "kubectl top node"
			_, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Runs an interactive command a pod", func() {
			cmd := "kubectl run busybox --rm -it --restart=Never --image=rancher/mirrored-library-busybox:1.36.1 -- uname -a"
			_, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Collects logs from a pod", func() {
			cmd := "kubectl logs -n kube-system -l k8s-app=metrics-server -c metrics-server"
			_, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Kills the cluster", func() {
			err := KillK3sCluster(append(tc.Servers, tc.Agents...))
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("Verify kubelet config file", func() {
		It("Starts K3s with no issues", func() {
			for _, node := range append(tc.Servers, tc.Agents...) {
				cmd := "mkdir -p --mode=0777 /tmp/kubelet.conf.d; echo 'apiVersion: kubelet.config.k8s.io/v1beta1\nkind: KubeletConfiguration\nshutdownGracePeriod: 19s\nshutdownGracePeriodCriticalPods: 13s' > /tmp/kubelet.conf.d/99-shutdownGracePeriod.conf"
				res, err := node.RunCmdOnNode(cmd)
				By("checking command results: " + res)
				Expect(err).NotTo(HaveOccurred())
			}

			kubeletConfigDirYAML := "kubelet-arg: config-dir=/tmp/kubelet.conf.d"
			err := StartK3sCluster(append(tc.Servers, tc.Agents...), kubeletConfigDirYAML, kubeletConfigDirYAML)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))

			By("CLUSTER CONFIG")
			By("OS:" + *nodeOS)
			By(tc.Status())
			tc.KubeconfigFile, err = e2e.GenKubeconfigFile(tc.Servers[0].String())
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status", func() {
			By("Fetching node status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "360s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "360s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(tc.KubeconfigFile)
			}, "300s", "10s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})

		It("Returns kubelet configuration", func() {
			for _, node := range append(tc.Servers, tc.Agents...) {
				cmd := "kubectl get --raw /api/v1/nodes/" + node.String() + "/proxy/configz"
				Expect(e2e.RunCommand(cmd)).To(ContainSubstring(`"shutdownGracePeriod":"19s","shutdownGracePeriodCriticalPods":"13s"`))
			}
		})

		It("Kills the cluster", func() {
			err := KillK3sCluster(append(tc.Servers, tc.Agents...))
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("Verify CRI-Dockerd", func() {
		It("Starts K3s with no issues", func() {
			dockerYAML := "docker: true"
			err := StartK3sCluster(append(tc.Servers, tc.Agents...), dockerYAML, dockerYAML)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))

			By("CLUSTER CONFIG")
			By("OS:" + *nodeOS)
			By(tc.Status())
			tc.KubeconfigFile, err = e2e.GenKubeconfigFile(tc.Servers[0].String())
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status", func() {
			By("Fetching node status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "360s", "5s").Should(Succeed())

			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "360s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(tc.KubeconfigFile)
			}, "300s", "10s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})
		It("Kills the cluster", func() {
			err := KillK3sCluster(append(tc.Servers, tc.Agents...))
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("Verify prefer-bundled-bin flag", func() {
		It("Starts K3s with no issues", func() {
			preferBundledYAML := "prefer-bundled-bin: true"
			err := StartK3sCluster(append(tc.Servers, tc.Agents...), preferBundledYAML, preferBundledYAML)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))

			By("CLUSTER CONFIG")
			By("OS:" + *nodeOS)
			By(tc.Status())
			tc.KubeconfigFile, err = e2e.GenKubeconfigFile(tc.Servers[0].String())
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status", func() {
			By("Fetching node status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "360s", "5s").Should(Succeed())

			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "360s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(tc.KubeconfigFile)
			}, "300s", "10s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})
		It("Kills the cluster", func() {
			err := KillK3sCluster(append(tc.Servers, tc.Agents...))
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("Verify disable-agent and egress-selector-mode flags", func() {
		It("Starts K3s with no issues", func() {
			disableAgentYAML := "disable-agent: true\negress-selector-mode: cluster"
			err := StartK3sCluster(append(tc.Servers, tc.Agents...), disableAgentYAML, "")
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))

			By("CLUSTER CONFIG")
			By("OS:" + *nodeOS)
			By(tc.Status())
			tc.KubeconfigFile, err = e2e.GenKubeconfigFile(tc.Servers[0].String())
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status", func() {
			By("Fetching node status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "360s", "5s").Should(Succeed())

			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "360s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
			Eventually(func() error {
				return tests.CheckDefaultDeployments(tc.KubeconfigFile)
			}, "300s", "10s").Should(Succeed())
		})

		It("Returns pod metrics", func() {
			cmd := "kubectl top pod -A"
			Eventually(func() error {
				_, err := e2e.RunCommand(cmd)
				return err
			}, "600s", "5s").Should(Succeed())
		})

		It("Returns node metrics", func() {
			cmd := "kubectl top node"
			_, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Runs an interactive command a pod", func() {
			cmd := "kubectl run busybox --rm -it --restart=Never --image=rancher/mirrored-library-busybox:1.36.1 -- uname -a"
			_, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Collects logs from a pod", func() {
			cmd := "kubectl logs -n kube-system -l app.kubernetes.io/name=traefik -c traefik"
			_, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Kills the cluster", func() {
			err := KillK3sCluster(append(tc.Servers, tc.Agents...))
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("Verify server picks up preloaded images on start", func() {
		It("Downloads and preloads images", func() {
			_, err := tc.Servers[0].RunCmdOnNode("docker pull ranchertest/mytestcontainer:latest")
			Expect(err).NotTo(HaveOccurred())
			_, err = tc.Servers[0].RunCmdOnNode("docker save ranchertest/mytestcontainer:latest -o /tmp/mytestcontainer.tar")
			Expect(err).NotTo(HaveOccurred())
			_, err = tc.Servers[0].RunCmdOnNode("mkdir -p /var/lib/rancher/k3s/agent/images/")
			Expect(err).NotTo(HaveOccurred())
			_, err = tc.Servers[0].RunCmdOnNode("mv /tmp/mytestcontainer.tar /var/lib/rancher/k3s/agent/images/")
			Expect(err).NotTo(HaveOccurred())
		})
		It("Starts K3s with no issues", func() {
			err := StartK3sCluster(append(tc.Servers, tc.Agents...), "", "")
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))

			By("CLUSTER CONFIG")
			By("OS:" + *nodeOS)
			By(tc.Status())
			tc.KubeconfigFile, err = e2e.GenKubeconfigFile(tc.Servers[0].String())
			Expect(err).NotTo(HaveOccurred())
		})
		It("has loaded the test container image", func() {
			Eventually(func() (string, error) {
				cmd := "k3s crictl images | grep ranchertest/mytestcontainer"
				return tc.Servers[0].RunCmdOnNode(cmd)
			}, "120s", "5s").Should(ContainSubstring("ranchertest/mytestcontainer"))
		})
		It("Kills the cluster", func() {
			err := KillK3sCluster(append(tc.Servers, tc.Agents...))
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("Verify server fails to start with bootstrap token", func() {
		It("Fails to start with a meaningful error", func() {
			tokenYAML := "token: aaaaaa.bbbbbbbbbbbbbbbb"
			err := StartK3sCluster(append(tc.Servers, tc.Agents...), tokenYAML, tokenYAML)
			Expect(err).To(HaveOccurred())
			Eventually(func(g Gomega) {
				logs, err := tc.Servers[0].GetJournalLogs()
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(logs).To(ContainSubstring("failed to normalize server token"))
			}, "120s", "5s").Should(Succeed())

		})
		It("Kills the cluster", func() {
			err := KillK3sCluster(append(tc.Servers, tc.Agents...))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		AddReportEntry("config", e2e.GetConfig(append(tc.Servers, tc.Agents...)))
		Expect(e2e.SaveJournalLogs(append(tc.Servers, tc.Agents...))).To(Succeed())
	} else {
		Expect(e2e.GetCoverageReport(append(tc.Servers, tc.Agents...))).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
