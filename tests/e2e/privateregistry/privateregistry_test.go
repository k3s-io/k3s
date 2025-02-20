package privateregistry

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
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
	RunSpecs(t, "Private Registry Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Create", Ordered, func() {
	Context("Cluster :", func() {
		It("Starts up with no issues", func() {
			var err error
			if *local {
				tc, err = e2e.CreateLocalCluster(*nodeOS, *serverCount, *agentCount)
			} else {
				tc, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
			}
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			By("CLUSTER CONFIG")
			By("OS: " + *nodeOS)
			By(tc.Status())

		})
		It("Checks node and pod status", func() {
			By("Fetching Nodes status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)

			By("Fetching pod status")
			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "620s", "10s").Should(Succeed())
		})

		It("Create new private registry", func() {
			registry, err := tc.Servers[0].RunCmdOnNode("docker run --init -d -p 5000:5000 --restart=always --name registry registry:2 ")
			fmt.Println(registry)
			Expect(err).NotTo(HaveOccurred())

		})
		It("ensures registry is working", func() {
			a, err := tc.Servers[0].RunCmdOnNode("docker ps -a | grep registry\n")
			fmt.Println(a)
			Expect(err).NotTo(HaveOccurred())

		})
		// Mirror the image as NODEIP:5000/docker-io-library/nginx:1.27.3, but reference it as my-registry.local/library/nginx:1.27.3 -
		// the rewrite in registries.yaml's entry for my-registry.local should ensure that it is rewritten properly when pulling from
		// NODEIP:5000 as a mirror.
		It("Should pull and image from dockerhub and send it to private registry", func() {
			cmd := "docker pull docker.io/library/nginx:1.27.3"
			_, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			nodeIP, err := tc.Servers[0].FetchNodeExternalIP()
			Expect(err).NotTo(HaveOccurred())

			cmd = "docker tag docker.io/library/nginx:1.27.3 " + nodeIP + ":5000/docker-io-library/nginx:1.27.3"
			_, err = tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			cmd = "docker push " + nodeIP + ":5000/docker-io-library/nginx:1.27.3"
			_, err = tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)

			cmd = "docker image remove docker.io/library/nginx:1.27.3 " + nodeIP + ":5000/docker-io-library/nginx:1.27.3"
			_, err = tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed: "+cmd)
		})

		It("Should create and validate deployment with private registry on", func() {
			res, err := tc.Servers[0].RunCmdOnNode("kubectl create deployment my-webpage --image=my-registry.local/library/nginx:1.27.3")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			var pod corev1.Pod
			Eventually(func(g Gomega) {
				pods, err := tests.ParsePods(tc.KubeconfigFile)
				for _, p := range pods {
					if strings.Contains(p.Name, "my-webpage") {
						pod = p
					}
				}
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(pod.Status.Phase)).Should(Equal("Running"))
			}, "60s", "5s").Should(Succeed())

			cmd := "curl -m 5 -s -f http://" + pod.Status.PodIP
			Expect(tc.Servers[0].RunCmdOnNode(cmd)).To(ContainSubstring("Welcome to nginx!"))
		})

	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		Expect(e2e.SaveJournalLogs(append(tc.Servers, tc.Agents...))).To(Succeed())
	} else {
		Expect(e2e.GetCoverageReport(append(tc.Servers, tc.Agents...))).To(Succeed())
	}
	if !failed || *ci {
		r1, err := tc.Servers[0].RunCmdOnNode("docker rm -f registry")
		Expect(err).NotTo(HaveOccurred(), r1)
		r2, err := tc.Servers[0].RunCmdOnNode("kubectl delete deployment my-webpage")
		Expect(err).NotTo(HaveOccurred(), r2)
		Expect(err).NotTo(HaveOccurred())
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
