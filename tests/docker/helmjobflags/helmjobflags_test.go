package main

import (
	"flag"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var ci = flag.Bool("ci", false, "running on CI, forced cleanup")

const (
	customHelmJobImage = "rancher/klipper-helm:v0.9.18-build20260428"
	helmSystemNS       = "kube-system"
	helmInstallJobName = "helm-install-traefik"
)

var config *docker.TestConfig

func Test_DockerHelmJobFlags(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Helm Job Flags Docker Test Suite")
}

var _ = Describe("Helm job flags", Ordered, func() {
	Context("Setup Cluster", func() {
		It("should provision a server with helm job flags", func() {
			var err error
			config, err = docker.NewTestConfig("rancher/systemd-node")
			Expect(err).NotTo(HaveOccurred())
			config.ServerYaml = `
helm-job-image: "` + customHelmJobImage + `"
helm-job-tolerations: '[{"key":"network","operator":"Exists","effect":"NoSchedule"}]'
`
			Expect(config.ProvisionServers(1)).To(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "120s", "5s").Should(Succeed())
		})
	})

	Context("Verify Helm Job", func() {
		It("creates the helm install traefik job", func() {
			Eventually(func() error {
				_, err := docker.GetJob(config.KubeconfigFile, helmSystemNS, helmInstallJobName)
				return err
			}, "180s", "5s").Should(Succeed())
		})

		It("uses the configured helm job image", func() {
			job, err := docker.GetJob(config.KubeconfigFile, helmSystemNS, helmInstallJobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(job.Spec.Template.Spec.Containers).ToNot(BeEmpty())
			Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal(customHelmJobImage))
		})

		It("applies the configured helm job toleration", func() {
			job, err := docker.GetJob(config.KubeconfigFile, helmSystemNS, helmInstallJobName)
			Expect(err).ToNot(HaveOccurred())
			Expect(docker.HasExpectedToleration(job.Spec.Template.Spec.Tolerations, "network", corev1.TolerationOpExists, corev1.TaintEffectNoSchedule)).To(BeTrue())
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
