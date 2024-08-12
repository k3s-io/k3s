package embeddedmirror

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
		It("Should create and validate deployment with embedded registry mirror using image tag", func() {
			res, err := e2e.RunCommand("kubectl create deployment my-webpage-1 --image=docker.io/library/nginx:1.25.3")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			patchCmd := fmt.Sprintf(`kubectl patch deployment my-webpage-1 --patch '{"spec":{"replicas":%d,"revisionHistoryLimit":0,"strategy":{"type":"Recreate", "rollingUpdate": null},"template":{"spec":{"affinity":{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchExpressions":[{"key":"app","operator":"In","values":["my-webpage-1"]}]},"topologyKey":"kubernetes.io/hostname"}]}}}}}}'`, *serverCount+*agentCount)
			res, err = e2e.RunCommand(patchCmd)
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl rollout status deployment my-webpage-1 --watch=true --timeout=360s")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should create and validate deployment with embedded registry mirror using image digest for existing tag", func() {
			res, err := e2e.RunCommand("kubectl create deployment my-webpage-2 --image=docker.io/library/nginx:nginx@sha256:c7a6ad68be85142c7fe1089e48faa1e7c7166a194caa9180ddea66345876b9d2")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			patchCmd := fmt.Sprintf(`kubectl patch deployment my-webpage-2 --patch '{"spec":{"replicas":%d,"revisionHistoryLimit":0,"strategy":{"type":"Recreate", "rollingUpdate": null},"template":{"spec":{"affinity":{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchExpressions":[{"key":"app","operator":"In","values":["my-webpage-2"]}]},"topologyKey":"kubernetes.io/hostname"}]}}}}}}'`, *serverCount+*agentCount)
			res, err = e2e.RunCommand(patchCmd)
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl rollout status deployment my-webpage-2 --watch=true --timeout=360s")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should create and validate deployment with embedded registry mirror using image digest without corresponding tag", func() {
			res, err := e2e.RunCommand("kubectl create deployment my-webpage-3 --image=docker.io/library/nginx@sha256:b4af4f8b6470febf45dc10f564551af682a802eda1743055a7dfc8332dffa595")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			patchCmd := fmt.Sprintf(`kubectl patch deployment my-webpage-3 --patch '{"spec":{"replicas":%d,"revisionHistoryLimit":0,"strategy":{"type":"Recreate", "rollingUpdate": null},"template":{"spec":{"affinity":{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchExpressions":[{"key":"app","operator":"In","values":["my-webpage-3"]}]},"topologyKey":"kubernetes.io/hostname"}]}}}}}}'`, *serverCount+*agentCount)
			res, err = e2e.RunCommand(patchCmd)
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl rollout status deployment my-webpage-3 --watch=true --timeout=360s")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should expose embedded registry metrics", func() {
			grepCmd := fmt.Sprintf("kubectl get --raw /api/v1/nodes/%s/proxy/metrics | grep -F 'spegel_advertised_images{registry=\"docker.io\"}'", serverNodeNames[0])
			res, err := e2e.RunCommand(grepCmd)
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Should cleanup deployments", func() {
			_, err := e2e.RunCommand("kubectl delete deployment my-webpage-1 my-webpage-2 my-webpage-3")
			Expect(err).NotTo(HaveOccurred())
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
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
