package upgrade

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	tester "github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Using these two flags, we upgrade from the latest release of <branch> to
// the current commit build of K3s defined by <k3sImage>
var k3sImage = flag.String("k3sImage", "", "The current commit build of K3s")
var channel = flag.String("channel", "latest", "The release channel to test")
var config *tester.TestConfig

var numServers = 1
var numAgents = 1

func Test_DockerUpgrade(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Upgrade Docker Test Suite")
}

var _ = Describe("Upgrade Tests", Ordered, func() {

	Context("Setup Cluster with Lastest Release", func() {
		var latestVersion string
		It("should determine latest branch version", func() {
			url := fmt.Sprintf("https://update.k3s.io/v1-release/channels/%s", *channel)
			resp, err := http.Head(url)
			// Cover the case where the branch does not exist yet,
			// such as a new unreleased minor version
			if err != nil || resp.StatusCode != http.StatusOK {
				*channel = "latest"
			}

			latestVersion, err = tester.GetVersionFromChannel(*channel)
			Expect(err).NotTo(HaveOccurred())
			Expect(latestVersion).To(ContainSubstring("v1."))
			fmt.Println("Using latest version: ", latestVersion)
		})
		It("should setup environment", func() {
			var err error
			config, err = tester.NewTestConfig("rancher/k3s:" + latestVersion)
			testID := filepath.Base(config.TestDir)
			Expect(err).NotTo(HaveOccurred())
			for i := 0; i < numServers; i++ {
				m1 := fmt.Sprintf("--mount type=volume,src=server-%d-%s-rancher,dst=/var/lib/rancher/k3s", i, testID)
				m2 := fmt.Sprintf("--mount type=volume,src=server-%d-%s-log,dst=/var/log", i, testID)
				m3 := fmt.Sprintf("--mount type=volume,src=server-%d-%s-etc,dst=/etc/rancher", i, testID)
				Expect(os.Setenv(fmt.Sprintf("SERVER_%d_DOCKER_ARGS", i), fmt.Sprintf("%s %s %s", m1, m2, m3))).To(Succeed())
			}
			for i := 0; i < numAgents; i++ {
				m1 := fmt.Sprintf("--mount type=volume,src=agent-%d-%s-rancher,dst=/var/lib/rancher/k3s", i, testID)
				m2 := fmt.Sprintf("--mount type=volume,src=agent-%d-%s-log,dst=/var/log", i, testID)
				m3 := fmt.Sprintf("--mount type=volume,src=agent-%d-%s-etc,dst=/etc/rancher", i, testID)
				Expect(os.Setenv(fmt.Sprintf("AGENT_%d_DOCKER_ARGS", i), fmt.Sprintf("%s %s %s", m1, m2, m3))).To(Succeed())
			}
		})
		It("should provision servers and agents", func() {
			Expect(config.ProvisionServers(numServers)).To(Succeed())
			Expect(config.ProvisionAgents(numAgents)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server", "traefik"}, config.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
		})
		It("should confirm latest version", func() {
			for _, server := range config.Servers {
				out, err := server.RunCmdOnNode("k3s --version")
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(ContainSubstring(strings.Replace(latestVersion, "-", "+", 1)))
			}
		})
		It("should deploy a test pod", func() {
			_, err := config.DeployWorkload("volume-test.yaml")
			Expect(err).NotTo(HaveOccurred(), "failed to apply volume test manifest")

			Eventually(func() (bool, error) {
				return tests.PodReady("volume-test", "kube-system", config.KubeconfigFile)
			}, "20s", "5s").Should(BeTrue())
		})
		It("should upgrade to current commit build", func() {
			By("Remove old servers and agents")
			for _, server := range config.Servers {
				cmd := fmt.Sprintf("docker stop %s", server.Name)
				Expect(tester.RunCommand(cmd)).Error().NotTo(HaveOccurred())
				cmd = fmt.Sprintf("docker rm %s", server.Name)
				Expect(tester.RunCommand(cmd)).Error().NotTo(HaveOccurred())
				fmt.Printf("Stopped %s\n", server.Name)
			}
			config.Servers = nil

			for _, agent := range config.Agents {
				cmd := fmt.Sprintf("docker stop %s", agent.Name)
				Expect(tester.RunCommand(cmd)).Error().NotTo(HaveOccurred())
				cmd = fmt.Sprintf("docker rm %s", agent.Name)
				Expect(tester.RunCommand(cmd)).Error().NotTo(HaveOccurred())
			}
			config.Agents = nil

			config.K3sImage = *k3sImage
			Expect(config.ProvisionServers(numServers)).To(Succeed())
			Expect(config.ProvisionAgents(numAgents)).To(Succeed())

			Eventually(func() error {
				return tests.CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server", "traefik"}, config.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
		})
		It("should confirm commit version", func() {
			for _, server := range config.Servers {
				Expect(tester.VerifyValidVersion(server, "kubectl")).To(Succeed())
				Expect(tester.VerifyValidVersion(server, "ctr")).To(Succeed())
				Expect(tester.VerifyValidVersion(server, "crictl")).To(Succeed())

				out, err := server.RunCmdOnNode("k3s --version")
				Expect(err).NotTo(HaveOccurred())
				cVersion := strings.Split(*k3sImage, ":")[1]
				cVersion = strings.Replace(cVersion, "-amd64", "", 1)
				cVersion = strings.Replace(cVersion, "-arm64", "", 1)
				cVersion = strings.Replace(cVersion, "-arm", "", 1)
				cVersion = strings.Replace(cVersion, "-", "+", 1)
				Expect(out).To(ContainSubstring(cVersion))
			}
		})
		It("should confirm test pod is still Running", func() {
			Eventually(func() (bool, error) {
				return tests.PodReady("volume-test", "kube-system", config.KubeconfigFile)
			}, "20s", "5s").Should(BeTrue())
		})

	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if config != nil && !failed {
		config.Cleanup()
	}
})
