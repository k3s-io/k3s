package main

import (
	"flag"
	"os"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k3sImage = flag.String("k3sImage", "", "The k3s image used to provision containers")
var ci = flag.Bool("ci", false, "running on CI, forced cleanup")
var config *docker.TestConfig

func Test_DockerT4(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "T4 Docker Test Suite")
}

var _ = Describe("T4 Tests", Ordered, func() {

	Context("Test a cluster with 1 server, no S3", func() {
		It("should setup the cluster configuration", func() {
			var err error
			config, err = docker.NewTestConfig(*k3sImage)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Setenv("SERVER_0_ARGS", "--datastore-endpoint=t4://")).To(Succeed())
		})
		It("should provision servers", func() {
			Expect(config.ProvisionServers(1)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(config.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "60s", "5s").Should(Succeed())
		})
		It("should destroy the cluster", func() {
			Expect(config.Cleanup()).To(Succeed())
		})
	})

	Context("Test a cluster with 2 servers, 1 agent, with S3", func() {
		It("should setup the cluster configuration", func() {
			var err error
			config, err = docker.NewTestConfig(*k3sImage)
			Expect(err).NotTo(HaveOccurred())
			config.DBType = "t4"
		})
		It("should provision servers and agents", func() {
			Expect(config.ProvisionServers(3)).To(Succeed())
			Expect(config.ProvisionAgents(1)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(config.KubeconfigFile)
			}, "90s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "90s", "5s").Should(Succeed())
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		AddReportEntry("describe", docker.DescribeNodesAndPods(config))
		AddReportEntry("docker-containers", docker.ListContainers())
		AddReportEntry("docker-logs", docker.TailDockerLogs(1000, append(config.Servers, config.Agents...)))
	}
	if config != nil && (*ci || !failed) {
		Expect(config.Cleanup()).To(Succeed())
	}
})
