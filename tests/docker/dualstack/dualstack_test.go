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

var (
	serverCount = flag.Int("serverCount", 3, "number of server nodes")
	ci          = flag.Bool("ci", false, "running on CI, force cleanup")
)

func Test_DockerDualStack(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	RunSpecs(t, "DualStack Docker Test Suite")
}

var _ = DescribeTableSubtree("DualStack Tests", Ordered, func(ipConfig string) {

	var (
		tc     *docker.TestConfig
		failed bool
	)

	Context("Setup Cluster", func() {
		It("should provision servers and agents", func() {
			var err error
			tc, err = docker.NewTestConfig("rancher/systemd-node")
			Expect(err).NotTo(HaveOccurred())
			tc.DualStack = true
			tc.ServerYaml = ipConfig
			Expect(tc.ProvisionServers(*serverCount)).To(Succeed())
			Expect(docker.RestartCluster(append(tc.Servers, tc.Agents...))).To(Succeed())
			Expect(tc.CopyAndModifyKubeconfig()).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(tests.CheckDefaultDeployments(tc.KubeconfigFile)).To(Succeed())
			}, "240s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})

	Context("Validate dualstack components", func() {
		It("Verifies that each node has IPv4 and IPv6", func() {
			for _, node := range append(tc.Servers, tc.Agents...) {
				ips, err := tests.GetNodeIPs(node.Name, tc.KubeconfigFile)
				Expect(err).NotTo(HaveOccurred(), "failed to get node IPs for "+node.Name)
				Expect(ips).To(ContainElements(ContainSubstring("172.18.0"), ContainSubstring("fd11:decf:c0ff")))
			}
		})
		It("Verifies that each pod has IPv4 and IPv6", func() {
			pods, err := tests.ParsePods(tc.KubeconfigFile, "kube-system")
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods {
				// Skip helm-install pods as the are in a Completed state and do not have IPs
				if strings.HasPrefix(pod.Name, "helm-install") {
					continue
				}
				ips, err := tests.GetPodIPs(pod.Name, pod.Namespace, tc.KubeconfigFile)
				Expect(err).NotTo(HaveOccurred(), "failed to get pod IPs for "+pod.Name)
				Expect(ips).NotTo(HaveLen(0), "failed to get pod IPs for "+pod.Name)
				Expect(ips).To(ContainElements(
					Or(ContainSubstring("172.18.0"), ContainSubstring("10.42.")),
					Or(ContainSubstring("fd11:decf:c0ff"), ContainSubstring("2001:cafe:42")),
				))
			}
		})

		It("Verifies ClusterIP Service", func() {
			_, err := tc.DeployWorkload("dualstack_clusterip.yaml")
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() (string, error) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				return tests.RunCommand(cmd)
			}, "120s", "5s").Should(ContainSubstring("ds-clusterip-pod"))

			// Checks both IPv4 and IPv6
			clusterips, err := tc.FetchClusterIP("ds-clusterip-svc")
			Expect(err).NotTo(HaveOccurred())
			for _, ip := range strings.Split(clusterips, ",") {
				if strings.Contains(ip, "::") {
					ip = "[" + ip + "]"
				}
				cmd := fmt.Sprintf("curl -L --insecure http://%s", ip)
				Eventually(func() (string, error) {
					return tc.Servers[0].RunCmdOnNode(cmd)
				}, "60s", "5s").Should(ContainSubstring("Welcome to nginx!"), "failed cmd: "+cmd)
			}
		})
		It("Verifies Ingress", func() {
			_, err := tc.DeployWorkload("dualstack_ingress.yaml")
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")
			cmd := "kubectl get ingress ds-ingress --kubeconfig=" + tc.KubeconfigFile + " -o jsonpath=\"{.spec.rules[*].host}\""
			hostName, err := tests.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
			for _, node := range append(tc.Servers, tc.Agents...) {
				ips, err := tests.GetNodeIPs(node.Name, tc.KubeconfigFile)
				Expect(err).NotTo(HaveOccurred(), "failed to get node IPs for "+node.Name)
				Expect(ips).To(HaveLen(2))
				for _, ip := range ips {
					if strings.Contains(ip, "::") {
						ip = "[" + ip + "]"
					}
					cmd := fmt.Sprintf("curl  --header host:%s http://%s/name.html", hostName, ip)
					Eventually(func() (string, error) {
						return tests.RunCommand(cmd)
					}, "10s", "2s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
				}
			}
		})

		It("Verifies NodePort Service", func() {
			_, err := tc.DeployWorkload("dualstack_nodeport.yaml")
			Expect(err).NotTo(HaveOccurred())
			cmd := "kubectl get service ds-nodeport-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport, err := tests.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
			for _, node := range append(tc.Servers, tc.Agents...) {
				ips, err := tests.GetNodeIPs(node.Name, tc.KubeconfigFile)
				Expect(err).NotTo(HaveOccurred(), "failed to get node IPs for "+node.Name)
				Expect(ips).To(HaveLen(2))
				for _, ip := range ips {
					if strings.Contains(ip, "::") {
						ip = "[" + ip + "]"
					}
					cmd = "curl -L --insecure http://" + ip + ":" + nodeport + "/name.html"
					Eventually(func() (string, error) {
						return tests.RunCommand(cmd)
					}, "10s", "1s").Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
				}
			}
		})
	})

	AfterAll(func() {
		failed = failed || CurrentSpecReport().Failed()
	})

	AfterAll(func() {
		if failed {
			AddReportEntry("describe", docker.DescribeNodesAndPods(tc))
			AddReportEntry("docker-containers", docker.ListContainers())
			logLen := 100
			if *ci {
				logLen = 1000
			}
			AddReportEntry("docker-logs", docker.TailDockerLogs(logLen, append(tc.Servers, tc.Agents...)))
		}
		if tc != nil && (*ci || !failed) {
			Expect(tc.Cleanup()).To(Succeed())
		}
	})
},
	Entry("ipv4,ipv6", "cluster-cidr: 10.42.0.0/16,2001:cafe:42:0::/56\nservice-cidr: 10.43.0.0/16,2001:cafe:43:0::/112"),
	Entry("ipv6,ipv4", "cluster-cidr: 2001:cafe:42:0::/56,10.42.0.0/16\nservice-cidr: 2001:cafe:43:0::/112,10.43.0.0/16"),
)
