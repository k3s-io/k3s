package main

import (
	"encoding/json"
	"flag"
	"fmt"
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

func Test_DockerEtcd(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Etcd Docker Test Suite")
}

var _ = Describe("Etcd Tests", Ordered, func() {

	Context("Test a 3 server cluster", func() {
		It("should setup the cluster configuration", func() {
			var err error
			config, err = docker.NewTestConfig(GinkgoTB(), *k3sImage)
			Expect(err).NotTo(HaveOccurred())
		})
		It("should provision servers", func() {
			Expect(config.ProvisionServers(3)).To(Succeed())
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

	Context("Test disabling etcd on a server in a 3 server cluster", func() {
		It("should setup the cluster configuration", func() {
			var err error
			config, err = docker.NewTestConfig(GinkgoTB(), "ghcr.io/rancher/systemd-node:v0.0.9")
			Expect(err).NotTo(HaveOccurred())
		})
		It("should provision servers", func() {
			Expect(config.ProvisionServers(3)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(config.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "60s", "5s").Should(Succeed())
		})
		It("should list all servers as etcd members", func() {
			Eventually(func() ([]string, error) {
				return etcdMemberNames(config.Servers[0])
			}, "60s", "5s").Should(HaveLen(3))
		})
		It("should remove the etcd member when etcd is disabled on a server", func() {
			server := config.Servers[1]
			_, err := server.RunCmdOnNode("echo 'disable-etcd: true' >> /etc/rancher/k3s/config.yaml")
			Expect(err).NotTo(HaveOccurred())
			Expect(docker.RestartCluster([]docker.DockerNode{server})).To(Succeed())
			Eventually(func() ([]string, error) {
				return etcdMemberNames(config.Servers[0])
			}, "120s", "5s").Should(And(HaveLen(2), Not(ContainElement(HavePrefix(server.Name)))))
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "120s", "5s").Should(Succeed())
		})
		It("should stay healthy when the server is restarted again", func() {
			server := config.Servers[1]
			Expect(docker.RestartCluster([]docker.DockerNode{server})).To(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "120s", "5s").Should(Succeed())
			Eventually(func() ([]string, error) {
				return etcdMemberNames(config.Servers[0])
			}, "60s", "5s").Should(And(HaveLen(2), Not(ContainElement(HavePrefix(server.Name)))))
		})
		It("should destroy the cluster", func() {
			Expect(config.Cleanup()).To(Succeed())
		})
	})

	Context("Test a Split Role cluster with 3 etcd, 2 control-plane, 1 agents", func() {
		It("should setup the cluster configuration", func() {
			var err error
			config, err = docker.NewTestConfig(GinkgoTB(), *k3sImage)
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
				return tests.CheckDefaultDeployments(config.KubeconfigFile)
			}, "240s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "90s", "5s").Should(Succeed())
		})
	})
})

// etcdMemberNames returns the names of the etcd members listed by the server's database info endpoint
func etcdMemberNames(server docker.DockerNode) ([]string, error) {
	out, err := tests.RunCommand(fmt.Sprintf("curl -ksSf -m 5 -u server:%s https://%s:6443/db/info", config.Token, server.IP))
	if err != nil {
		return nil, err
	}
	var info struct {
		Members []struct {
			Name string `json:"name"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(info.Members))
	for _, member := range info.Members {
		names = append(names, member.Name)
	}
	return names, nil
}

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
