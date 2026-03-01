package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k3sImage = flag.String("k3sImage", "", "The k3s image used to provision containers")
var ci = flag.Bool("ci", false, "running on CI, forced cleanup")
var config *docker.TestConfig

func Test_DockerNixSnapshotter(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Nix Snapshotter Docker Test Suite")
}

var _ = Describe("Nix Snapshotter Tests", Ordered, func() {

	Context("Setup Cluster", func() {
		It("should provision servers with nix snapshotter", func() {
			var err error
			config, err = docker.NewTestConfig(*k3sImage)
			Expect(err).NotTo(HaveOccurred())

			// Write a container entrypoint wrapper that symlinks nix-store
			// into the PATH before starting k3s. This is needed because the
			// NixSupported() check calls exec.LookPath("nix-store") during
			// startup, before we have a chance to docker exec into the container.
			entrypoint := filepath.Join(config.TestDir, "nix-entrypoint.sh")
			Expect(os.WriteFile(entrypoint, []byte("#!/bin/sh\nln -sf /nix/var/nix/profiles/default/bin/nix-store /usr/local/bin/nix-store\nexec /bin/k3s \"$@\"\n"), 0755)).To(Succeed())

			os.Setenv("SERVER_DOCKER_ARGS", fmt.Sprintf("--restart=always -v /nix:/nix -v %s:/usr/local/bin/nix-entrypoint.sh --entrypoint /usr/local/bin/nix-entrypoint.sh", entrypoint))

			config.ServerYaml = "snapshotter: nix"
			Expect(config.ProvisionServers(1)).To(Succeed())

			Eventually(func() error {
				return tests.CheckDefaultDeployments(config.KubeconfigFile)
			}, "180s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})

	Context("Verify Nix Snapshotter", func() {
		It("should run a pod using a nix-built image", func() {
			// Copy the nix test image OCI tar into the k3s container.
			// Built by CI via: nix build github:pdtpartners/nix-snapshotter#image-hello
			cmd := fmt.Sprintf("docker cp ../resources/nix-hello-image.tar %s:/tmp/nix-hello-image.tar", config.Servers[0].Name)
			_, err := tests.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to copy test image into container")

			// Run a pod using the nix:0 image reference prefix. This directs
			// kubelet's PullImage through the nix-snapshotter image service,
			// which loads the OCI tar and unpacks layers with nix-closure
			// annotation processing. The snapshotter's Prepare() realizes
			// the closure's nix store paths via nix-store and creates GC
			// roots. The image's entrypoint (hello) prints and exits 0.
			cmd = fmt.Sprintf("kubectl --kubeconfig=%s run nix-hello --image=nix:0/tmp/nix-hello-image.tar --image-pull-policy=Always --restart=Never", config.KubeconfigFile)
			_, err = tests.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to create nix-hello pod")

			// Wait for the pod to complete successfully.
			Eventually(func() (string, error) {
				cmd := fmt.Sprintf("kubectl --kubeconfig=%s get pod nix-hello -o jsonpath='{.status.phase}'", config.KubeconfigFile)
				return tests.RunCommand(cmd)
			}, "60s", "5s").Should(Equal("Succeeded"))
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

