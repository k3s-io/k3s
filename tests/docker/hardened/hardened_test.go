package main

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var config *docker.TestConfig
var ci = flag.Bool("ci", false, "running on CI")

func Test_DockerHardened(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hardened Docker Test Suite")
}

var _ = Describe("Hardened Tests", Ordered, func() {

	Context("Setup Cluster", func() {
		It("should provision servers and agents", func() {
			var err error
			config, err = docker.NewTestConfig("rancher/systemd-node")
			Expect(err).NotTo(HaveOccurred())
			config.ServerYaml = `
secrets-encryption: true
kube-controller-manager-arg:
  - 'terminated-pod-gc-threshold=10'
kubelet-arg:
  - 'streaming-connection-idle-timeout=5m'
  - 'make-iptables-util-chains=true'
  - 'event-qps=0'
  - "tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305"
kube-apiserver-arg:
  - 'admission-control-config-file=/tmp/cluster-level-pss.yaml'
  - 'audit-log-path=/var/lib/rancher/k3s/server/logs/audit.log'
  - 'audit-policy-file=/var/lib/rancher/k3s/server/audit.yaml'
  - 'audit-log-maxage=30'
  - 'audit-log-maxbackup=10'
  - 'audit-log-maxsize=100'
`
			config.AgentYaml = `
kubelet-arg:
  - 'streaming-connection-idle-timeout=5m'
  - "tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305"
`
			config.SkipStart = true
			Expect(config.ProvisionServers(1)).To(Succeed())

			for _, server := range config.Servers {
				cmd := "docker cp ./cluster-level-pss.yaml " + server.Name + ":/tmp/cluster-level-pss.yaml"
				Expect(tests.RunCommand(cmd)).Error().NotTo(HaveOccurred())

				cmd = "mkdir -p /var/lib/rancher/k3s/server/logs"
				Expect(server.RunCmdOnNode(cmd)).Error().NotTo(HaveOccurred())
				auditYaml := "apiVersion: audit.k8s.io/v1\nkind: Policy\nrules:\n- level: Metadata"
				cmd = fmt.Sprintf("echo -e '%s' > /var/lib/rancher/k3s/server/audit.yaml", auditYaml)
				Expect(server.RunCmdOnNode(cmd)).Error().NotTo(HaveOccurred())
				Expect(server.RunCmdOnNode("systemctl start k3s")).Error().NotTo(HaveOccurred())
			}
			Expect(config.CopyAndModifyKubeconfig()).To(Succeed())
			config.SkipStart = false
			Expect(config.ProvisionAgents(1)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(config.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})

	Context("Verify Network Policies", func() {
		It("applies network policies", func() {
			_, err := config.DeployWorkload("hardened-ingress.yaml")
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() (int, error) {
				return tests.GetDaemonsetReady("example", config.KubeconfigFile)
			}, "60s", "5s").Should(Equal(2))
			_, err = config.DeployWorkload("hardened-netpool.yaml")
			Expect(err).NotTo(HaveOccurred())
		})
		It("checks ingress connections", func() {
			for _, scheme := range []string{"http", "https"} {
				Eventually(func(g Gomega) {
					for _, server := range config.Servers {
						cmd := fmt.Sprintf("curl -vksf -m 5 -H 'Host: example.com' %s://%s/", scheme, server.IP)
						g.Expect(tests.RunCommand(cmd)).Error().NotTo(HaveOccurred())
					}
					for _, agent := range config.Agents {
						cmd := fmt.Sprintf("curl -vksf -m 5 -H 'Host: example.com' %s://%s/", scheme, agent.IP)
						g.Expect(tests.RunCommand(cmd)).Error().NotTo(HaveOccurred())
					}
				}, "30s", "10s").Should(Succeed())
			}
		})
		It("confirms we can make a request through the nodeport service", func() {
			for _, server := range config.Servers {
				cmd := "kubectl get service/example -o 'jsonpath={.spec.ports[*].nodePort}'"
				ports, err := tests.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
				for _, port := range strings.Split(ports, " ") {
					cmd := fmt.Sprintf("curl -vksf -m 5 -H 'Host: example.com' http://%s:%s", server.IP, port)
					Expect(tests.RunCommand(cmd)).Error().NotTo(HaveOccurred())
				}
			}
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if *ci || (config != nil && !failed) {
		Expect(config.Cleanup()).To(Succeed())
	}
})
