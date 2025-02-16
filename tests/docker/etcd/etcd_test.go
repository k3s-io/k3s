package main

import (
	"flag"
	"os"
	"testing"

	"github.com/k3s-io/k3s/tests"
	tester "github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k3sImage = flag.String("k3sImage", "", "The k3s image used to provision containers")
var config *tester.TestConfig

func Test_DockerEtcd(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Etcd Docker Test Suite")
}

var _ = Describe("Etcd Tests", Ordered, func() {

	Context("Test a 3 server cluster", func() {
		It("should setup the cluster configuration", func() {
			var err error
			config, err = tester.NewTestConfig(*k3sImage)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should provision servers", func() {
			Expect(config.ProvisionServers(3)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server", "traefik"}, config.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "60s", "5s").Should(Succeed())
		})
		It("should destroy the cluster", func() {
			Expect(config.Cleanup()).To(Succeed())
		})
	})

	Context("Test a Split Role cluster with 3 etcd, 2 control-plane, 1 agents", func() {
		It("should setup the cluster configuration", func() {
			var err error
			config, err = tester.NewTestConfig(*k3sImage)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Setenv("SERVER_0_ARGS", "--disable-apiserver --disable-controller-manager --disable-scheduler --cluster-init")).To(Succeed())
			Expect(os.Setenv("SERVER_1_ARGS", "--disable-apiserver --disable-controller-manager --disable-scheduler")).To(Succeed())
			Expect(os.Setenv("SERVER_2_ARGS", "--disable-apiserver --disable-controller-manager --disable-scheduler")).To(Succeed())
			Expect(os.Setenv("SERVER_3_ARGS", "--disable-etcd")).To(Succeed())
			Expect(os.Setenv("SERVER_4_ARGS", "--disable-etcd")).To(Succeed())
		})
		It("should provision servers and agents", func() {
			Expect(config.ProvisionServers(5)).To(Succeed())
			Expect(config.ProvisionAgents(1)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server", "traefik"}, config.KubeconfigFile)
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
	if config != nil && !failed {
		config.Cleanup()
	}
})
