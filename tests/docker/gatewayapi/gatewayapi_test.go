package main

import (
	"flag"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k3sImage = flag.String("k3sImage", "", "The image used to provision containers")
var ci = flag.Bool("ci", false, "running on CI, forced cleanup")
var config *docker.TestConfig

func Test_DockerGatewayAPI(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gateway API Docker Test Suite")
}

var _ = Describe("Gateway API Tests", Ordered, func() {

	Context("Setup Cluster", func() {
		It("should provision a server", func() {
			var err error
			config, err = docker.NewTestConfig(GinkgoTB(), *k3sImage)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ProvisionServers(1)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(config.KubeconfigFile)
			}, "180s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})

	Context("Enable Traefik's Gateway API provider", func() {
		It("should install the Gateway API CRDs from the bundled gateway-api-crd manifest", func() {
			cmd := "kubectl get crd gateways.gateway.networking.k8s.io httproutes.gateway.networking.k8s.io gatewayclasses.gateway.networking.k8s.io --kubeconfig=" + config.KubeconfigFile
			Eventually(func() (string, error) {
				return tests.RunCommand(cmd)
			}, "120s", "5s").Should(ContainSubstring("gatewayclasses.gateway.networking.k8s.io"), "failed cmd: "+cmd)
		})
		It("should apply a HelmChartConfig enabling the kubernetesGateway provider", func() {
			_, err := config.DeployWorkload("gatewayapi-helmchartconfig.yaml")
			Expect(err).NotTo(HaveOccurred(), "failed to apply gateway API HelmChartConfig")
		})
		It("should accept the traefik GatewayClass", func() {
			cmd := "kubectl get gatewayclass traefik -o jsonpath='{.status.conditions[?(@.type==\"Accepted\")].status}' --kubeconfig=" + config.KubeconfigFile
			Eventually(func() (string, error) {
				return tests.RunCommand(cmd)
			}, "120s", "5s").Should(Equal("True"), "failed cmd: "+cmd)
		})
	})

	Context("Validate Gateway API routing", func() {
		It("should deploy a backend and an HTTPRoute", func() {
			_, err := config.DeployWorkload("gatewayapi.yaml")
			Expect(err).NotTo(HaveOccurred(), "failed to apply gateway API workload")
		})
		It("should accept the HTTPRoute", func() {
			cmd := "kubectl get httproute gatewayapi-echo -n kube-system -o jsonpath='{.status.parents[0].conditions[?(@.type==\"Accepted\")].status}' --kubeconfig=" + config.KubeconfigFile
			Eventually(func() (string, error) {
				return tests.RunCommand(cmd)
			}, "120s", "5s").Should(Equal("True"), "failed cmd: "+cmd)
		})
		It("should route HTTP traffic through the gateway", func() {
			cmd := "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -H 'Host: gatewayapi.example.com' http://" + config.Servers[0].IP
			Eventually(func() (string, error) {
				return tests.RunCommand(cmd)
			}, "120s", "5s").Should(Equal("200"), "failed cmd: "+cmd)
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
		AddReportEntry("docker-logs", docker.TailDockerLogs(1000, config.Servers))
	}
	if config != nil && (*ci || !failed) {
		Expect(config.Cleanup()).To(Succeed())
	}
})
