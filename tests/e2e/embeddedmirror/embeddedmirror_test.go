package embeddedmirror

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/k3s-io/k3s/tests"
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
// E2E_RELEASE_VERSION=v1.23.1+k3s2 (default: latest commit from main)
// E2E_REGISTRY: true/false (default: false)

func Test_E2EEmbeddedMirror(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Embedded Mirror Test Suite", suiteConfig, reporterConfig)
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
		It("Saves image into server images dir", func() {
			res, err := e2e.RunCommand("docker image pull docker.io/rancher/mirrored-library-busybox:1.34.1")
			Expect(err).NotTo(HaveOccurred(), "failed to pull image: "+res)
			res, err = e2e.RunCommand("docker image tag docker.io/rancher/mirrored-library-busybox:1.34.1 registry.example.com/rancher/mirrored-library-busybox:1.34.1")
			Expect(err).NotTo(HaveOccurred(), "failed to tag image: "+res)
			res, err = e2e.RunCommand("docker image save registry.example.com/rancher/mirrored-library-busybox:1.34.1 -o mirrored-library-busybox.tar")
			Expect(err).NotTo(HaveOccurred(), "failed to save image: "+res)
			res, err = e2e.RunCommand("vagrant scp mirrored-library-busybox.tar " + tc.Servers[0].String() + ":/tmp/mirrored-library-busybox.tar")
			Expect(err).NotTo(HaveOccurred(), "failed to 'vagrant scp' image tarball: "+res)
			res, err = tc.Servers[0].RunCmdOnNode("mv /tmp/mirrored-library-busybox.tar /var/lib/rancher/k3s/agent/images/mirrored-library-busybox.tar")
			Expect(err).NotTo(HaveOccurred(), "failed to move image tarball: "+res)
		})
		It("Checks node and pod status", func() {
			By("Fetching Nodes status")
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, e2e.VagrantSlice(tc.AllNodes()))
			}, "620s", "5s").Should(Succeed())

			By("Fetching pod status")
			Eventually(func() error {
				e2e.DumpPods(tc.KubeconfigFile)
				return tests.AllPodsUp(tc.KubeconfigFile, "kube-system")
			}, "620s", "10s").Should(Succeed())
		})
		It("Should create and validate deployment with embedded registry mirror using image tag", func() {
			res, err := e2e.RunCommand("kubectl create deployment my-deployment-1 --image=docker.io/rancher/mirrored-library-busybox:1.37.0 -- sleep 86400")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			patchCmd := fmt.Sprintf(`kubectl patch deployment my-deployment-1 --patch '{"spec":{"replicas":%d,"revisionHistoryLimit":0,"strategy":{"type":"Recreate", "rollingUpdate": null},"template":{"spec":{"affinity":{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchExpressions":[{"key":"app","operator":"In","values":["my-deployment-1"]}]},"topologyKey":"kubernetes.io/hostname"}]}}}}}}'`, *serverCount+*agentCount)
			res, err = e2e.RunCommand(patchCmd)
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl rollout status deployment my-deployment-1 --watch=true --timeout=360s")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl delete deployment my-deployment-1")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())
		})

		// @sha256:101b4afd76732482eff9b95cae5f94bcf295e521fbec4e01b69c5421f3f3f3e5 is :1.37.0 which has already been pulled and should be reused
		It("Should create and validate deployment with embedded registry mirror using image digest for existing tag", func() {
			res, err := e2e.RunCommand("kubectl create deployment my-deployment-2 --image=docker.io/rancher/mirrored-library-busybox@sha256:101b4afd76732482eff9b95cae5f94bcf295e521fbec4e01b69c5421f3f3f3e5 -- sleep 86400")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			patchCmd := fmt.Sprintf(`kubectl patch deployment my-deployment-2 --patch '{"spec":{"replicas":%d,"revisionHistoryLimit":0,"strategy":{"type":"Recreate", "rollingUpdate": null},"template":{"spec":{"affinity":{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchExpressions":[{"key":"app","operator":"In","values":["my-deployment-2"]}]},"topologyKey":"kubernetes.io/hostname"}]}}}}}}'`, *serverCount+*agentCount)
			res, err = e2e.RunCommand(patchCmd)
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl rollout status deployment my-deployment-2 --watch=true --timeout=360s")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl delete deployment my-deployment-2")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())
		})

		// @sha256:8a45424ddf949bbe9bb3231b05f9032a45da5cd036eb4867b511b00734756d6f is :1.36.1 which should not have been pulled yet
		It("Should create and validate deployment with embedded registry mirror using image digest without existing tag", func() {
			res, err := e2e.RunCommand("kubectl create deployment my-deployment-3 --image=docker.io/rancher/mirrored-library-busybox@sha256:8a45424ddf949bbe9bb3231b05f9032a45da5cd036eb4867b511b00734756d6f -- sleep 86400")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			patchCmd := fmt.Sprintf(`kubectl patch deployment my-deployment-3 --patch '{"spec":{"replicas":%d,"revisionHistoryLimit":0,"strategy":{"type":"Recreate", "rollingUpdate": null},"template":{"spec":{"affinity":{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchExpressions":[{"key":"app","operator":"In","values":["my-deployment-3"]}]},"topologyKey":"kubernetes.io/hostname"}]}}}}}}'`, *serverCount+*agentCount)
			res, err = e2e.RunCommand(patchCmd)
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl rollout status deployment my-deployment-3 --watch=true --timeout=360s")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl delete deployment my-deployment-3")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())
		})

		// create deployment from imported image
		It("Should create and validate deployment with embedded registry mirror using image tag from import", func() {
			res, err := e2e.RunCommand("kubectl create deployment my-deployment-4 --image=registry.example.com/rancher/mirrored-library-busybox:1.34.1 -- sleep 86400")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			patchCmd := fmt.Sprintf(`kubectl patch deployment my-deployment-4 --patch '{"spec":{"replicas":%d,"revisionHistoryLimit":0,"strategy":{"type":"Recreate", "rollingUpdate": null},"template":{"spec":{"affinity":{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchExpressions":[{"key":"app","operator":"In","values":["my-deployment-4"]}]},"topologyKey":"kubernetes.io/hostname"}]}}}}}}'`, *serverCount+*agentCount)
			res, err = e2e.RunCommand(patchCmd)
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl rollout status deployment my-deployment-4 --watch=true --timeout=360s")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl delete deployment my-deployment-4")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())
		})

		// @sha256:125dfcbe72a0158c16781d3ad254c0d226a6534b59cc7c2bf549cdd50c6e8989 is :1.34.1 which should have been created by the image import
		// Note that the digest here may vary depending on the image store used to save the image. When using docker with
		// containerd-snapshotter, image pull/save retains the original manifest list and digest, but the legacy docker
		// snapshotter does not and will flatten the manifest list to a single-platform image with a different digest.
		// If this test fails, make sure the `docker image save` command above is run on a host that is using containerd-snapshotter.
		It("Should create and validate deployment with embedded registry mirror using image digest from import", func() {
			res, err := e2e.RunCommand("kubectl create deployment my-deployment-5 --image=registry.example.com/rancher/mirrored-library-busybox@sha256:125dfcbe72a0158c16781d3ad254c0d226a6534b59cc7c2bf549cdd50c6e8989 -- sleep 86400")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			patchCmd := fmt.Sprintf(`kubectl patch deployment my-deployment-5 --patch '{"spec":{"replicas":%d,"revisionHistoryLimit":0,"strategy":{"type":"Recreate", "rollingUpdate": null},"template":{"spec":{"affinity":{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchExpressions":[{"key":"app","operator":"In","values":["my-deployment-5"]}]},"topologyKey":"kubernetes.io/hostname"}]}}}}}}'`, *serverCount+*agentCount)
			res, err = e2e.RunCommand(patchCmd)
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl rollout status deployment my-deployment-5 --watch=true --timeout=360s")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())

			res, err = e2e.RunCommand("kubectl delete deployment my-deployment-5")
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())
		})

		/* Disabled, ref: https://github.com/spegel-org/spegel/issues/1023
		It("Should expose embedded registry metrics", func() {
			grepCmd := fmt.Sprintf("kubectl get --raw /api/v1/nodes/%s/proxy/metrics | grep -F 'spegel_advertised_images{registry=\"docker.io\"}'", tc.Servers[0])
			res, err := e2e.RunCommand(grepCmd)
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())
		})
		*/
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		Expect(e2e.SaveJournalLogs(tc.AllNodes())).To(Succeed())
		Expect(e2e.TailPodLogs(50, tc.AllNodes())).To(Succeed())
	} else {
		Expect(e2e.GetCoverageReport(tc.AllNodes())).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
