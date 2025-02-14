package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	tester "github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k3sImage = flag.String("k3sImage", "", "The k3s image used to provision containers")
var config *tester.TestConfig

func Test_DockerBootstrapToken(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "BoostrapToken Docker Test Suite")
}

var _ = Describe("Boostrap Token Tests", Ordered, func() {

	Context("Setup Cluster", func() {
		It("should provision servers", func() {
			var err error
			config, err = tester.NewTestConfig(*k3sImage)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ProvisionServers(1)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server", "traefik"}, config.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
		})
	})

	Context("Add Agent with Bootstrap token", func() {
		var newSecret string
		It("creates a bootstrap token", func() {
			var err error
			newSecret, err = config.Servers[0].RunCmdOnNode("k3s token create --ttl=5m --description=Test")
			Expect(err).NotTo(HaveOccurred())
			Expect(newSecret).NotTo(BeEmpty())
		})
		It("joins the agent with the new tokens", func() {
			newSecret = strings.ReplaceAll(newSecret, "\n", "")
			config.Token = newSecret
			Expect(config.ProvisionAgents(1)).To(Succeed())
			Eventually(func(g Gomega) {
				nodes, err := tests.ParseNodes(config.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(nodes).To(HaveLen(2))
				g.Expect(tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())).To(Succeed())
			}, "40s", "5s").Should(Succeed())
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
