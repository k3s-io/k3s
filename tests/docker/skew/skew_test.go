package main

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	tester "github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Using these two flags, we upgrade from the latest release of <branch> to
// the current commit build of K3s defined by <k3sImage>
var k3sImage = flag.String("k3sImage", "", "The current commit build of K3s")
var branch = flag.String("branch", "master", "The release branch to test")
var config *tester.TestConfig

var numServers = 1
var numAgents = 1

func Test_DockerSkew(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Skew Docker Test Suite")
}

var lastMinorVersion string
var _ = BeforeSuite(func() {
	// If this test runs on v1.31 commit, we want the latest v1.30 release
	// For master and unreleased branches, we want the latest stable release
	var upgradeChannel string
	var err error
	if *branch == "master" || *branch == "release-1.32" {
		// disabled: AuthorizeNodeWithSelectors is now on by default, which breaks compat with agents < v1.32.
		// This can be ren-enabled once the previous branch is v1.32 or higher, or when RBAC changes have been backported.
		// ref: https://github.com/kubernetes/kubernetes/pull/128168
		Skip("Skipping version skew tests for " + *branch + " due to AuthorizeNodeWithSelectors")

		upgradeChannel = "stable"
	} else {
		upgradeChannel = strings.Replace(*branch, "release-", "v", 1)
		// now that it is in v1.1 format, we want to substract one from the minor version
		// to get the previous release
		sV, err := semver.ParseTolerant(upgradeChannel)
		Expect(err).NotTo(HaveOccurred(), "failed to parse version from "+upgradeChannel)
		sV.Minor--
		upgradeChannel = fmt.Sprintf("v%d.%d", sV.Major, sV.Minor)
	}

	lastMinorVersion, err = tester.GetVersionFromChannel(upgradeChannel)
	Expect(err).NotTo(HaveOccurred())
	Expect(lastMinorVersion).To(ContainSubstring("v1."))

	fmt.Println("Using last minor version: ", lastMinorVersion)
})

var _ = Describe("Skew Tests", Ordered, func() {
	Context("Setup Cluster with Server newer than Agent", func() {
		It("should provision new servers and old agents", func() {
			var err error
			config, err = tester.NewTestConfig(*k3sImage)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ProvisionServers(numServers)).To(Succeed())
			config.K3sImage = "rancher/k3s:" + lastMinorVersion
			Expect(config.ProvisionAgents(numAgents)).To(Succeed())
			Eventually(func() error {
				return tester.DeploymentsReady([]string{"coredns", "local-path-provisioner", "metrics-server", "traefik"}, config.KubeconfigFile)
			}, "60s", "5s").Should(Succeed())
		})
		It("should match respective versions", func() {
			for _, server := range config.Servers {
				out, err := tester.RunCmdOnDocker(server.Name, "k3s --version")
				Expect(err).NotTo(HaveOccurred())
				// The k3s image is in the format rancher/k3s:v1.20.0-k3s1
				cVersion := strings.Split(*k3sImage, ":")[1]
				cVersion = strings.Replace(cVersion, "-amd64", "", 1)
				cVersion = strings.Replace(cVersion, "-", "+", 1)
				Expect(out).To(ContainSubstring(cVersion))
			}
			for _, agent := range config.Agents {
				Expect(tester.RunCmdOnDocker(agent.Name, "k3s --version")).
					To(ContainSubstring(strings.Replace(lastMinorVersion, "-", "+", 1)))
			}
		})
		It("should deploy a test pod", func() {
			const volumeTestManifest = "../resources/volume-test.yaml"

			// Apply the manifest
			cmd := fmt.Sprintf("kubectl apply -f %s --kubeconfig=%s", volumeTestManifest, config.KubeconfigFile)
			_, err := tester.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to apply volume test manifest")

			Eventually(func() (bool, error) {
				return tester.PodReady("volume-test", "kube-system", config.KubeconfigFile)
			}, "20s", "5s").Should(BeTrue())
		})
		It("should destroy the cluster", func() {
			Expect(config.Cleanup()).To(Succeed())
		})
	})
	Context("Test cluster with 1 Server older and 2 Servers newer", func() {
		It("should setup the cluster configuration", func() {
			var err error
			config, err = tester.NewTestConfig("rancher/k3s:" + lastMinorVersion)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should provision servers", func() {
			Expect(config.ProvisionServers(1)).To(Succeed())
			config.K3sImage = *k3sImage
			Expect(config.ProvisionServers(3)).To(Succeed())
			Eventually(func() error {
				return tester.DeploymentsReady([]string{"coredns", "local-path-provisioner", "metrics-server", "traefik"}, config.KubeconfigFile)
			}, "60s", "5s").Should(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(tester.ParseNodes(config.KubeconfigFile)).To(HaveLen(3))
				g.Expect(tester.NodesReady(config.KubeconfigFile)).To(Succeed())
			}, "60s", "5s").Should(Succeed())
		})
		It("should match respective versions", func() {
			out, err := tester.RunCmdOnDocker(config.Servers[0].Name, "k3s --version")
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(ContainSubstring(strings.Replace(lastMinorVersion, "-", "+", 1)))
			for _, server := range config.Servers[1:] {
				out, err := tester.RunCmdOnDocker(server.Name, "k3s --version")
				Expect(err).NotTo(HaveOccurred())
				// The k3s image is in the format rancher/k3s:v1.20.0-k3s1-amd64
				cVersion := strings.Split(*k3sImage, ":")[1]
				cVersion = strings.Replace(cVersion, "-amd64", "", 1)
				cVersion = strings.Replace(cVersion, "-", "+", 1)
				Expect(out).To(ContainSubstring(cVersion))
			}
		})
		It("should destroy the cluster", func() {
			Expect(config.Cleanup()).To(Succeed())
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
