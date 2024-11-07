package privateregistry

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS:
// bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
// eurolinux-vagrant/rocky-8, eurolinux-vagrant/rocky-9,
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 1, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 (default: latest commit from master)
// E2E_REGISTRY: true/false (default: false)

func Test_E2EPrivateRegistry(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Create Cluster Test Suite", suiteConfig, reporterConfig)
}

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Create", Ordered, func() {
	Context("Cluster :", func() {
		It("Starts up with no issues", func() {
			var err error
			if *local {
				serverNodeNames, agentNodeNames, err = e2e.CreateLocalCluster(*nodeOS, *serverCount, *agentCount)
			} else {
				serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
			}
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			fmt.Println("CLUSTER CONFIG")
			fmt.Println("OS:", *nodeOS)
			fmt.Println("Server Nodes:", serverNodeNames)
			fmt.Println("Agent Nodes:", agentNodeNames)
			kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
		})
		It("Checks Node and Pod Status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(kubeConfigFile, true)

			fmt.Printf("\nFetching Pods status\n")
			Eventually(func(g Gomega) {
				pods, err := e2e.ParsePods(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
					} else {
						g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
					}
				}
			}, "620s", "5s").Should(Succeed())
			_, _ = e2e.ParsePods(kubeConfigFile, true)
		})

		It("Create new private registry", func() {
			registry, err := e2e.RunCmdOnNode("docker run --init -d -p 5000:5000 --restart=always --name registry registry:2 ", serverNodeNames[0])
			fmt.Println(registry)
			Expect(err).NotTo(HaveOccurred())

		})
		It("ensures registry is working", func() {
			a, err := e2e.RunCmdOnNode("docker ps -a | grep registry\n", serverNodeNames[0])
			fmt.Println(a)
			Expect(err).NotTo(HaveOccurred())

		})
		It("Should pull and image from dockerhub and send it to private registry", func() {
			cmd := "docker pull nginx"
			_, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			nodeIP, err := e2e.FetchNodeExternalIP(serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())

			cmd = "docker tag nginx " + nodeIP + ":5000/my-webpage"
			_, err = e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			cmd = "docker push " + nodeIP + ":5000/my-webpage"
			_, err = e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			cmd = "docker image remove nginx " + nodeIP + ":5000/my-webpage"
			_, err = e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)
		})
		It("Should create and validate deployment with private registry on", func() {
			res, err := e2e.RunCmdOnNode("kubectl create deployment my-webpage --image=my-registry.local/my-webpage", serverNodeNames[0])
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			var pod e2e.Pod
			Eventually(func(g Gomega) {
				pods, err := e2e.ParsePods(kubeConfigFile, false)
				for _, p := range pods {
					if strings.Contains(p.Name, "my-webpage") {
						pod = p
					}
				}
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(pod.Status).Should(Equal("Running"))
			}, "60s", "5s").Should(Succeed())

			cmd := "curl " + pod.IP
			Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).To(ContainSubstring("Welcome to nginx!"))
		})

	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {

	if !failed {
		Expect(e2e.GetCoverageReport(append(serverNodeNames, agentNodeNames...))).To(Succeed())
	}
	if !failed || *ci {
		r1, err := e2e.RunCmdOnNode("docker rm -f registry", serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred(), r1)
		r2, err := e2e.RunCmdOnNode("kubectl delete deployment my-webpage", serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred(), r2)
		Expect(err).NotTo(HaveOccurred())
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
