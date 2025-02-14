package main

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	tester "github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k3sImage = flag.String("k3sImage", "", "The k3s image used to provision containers")
var config *tester.TestConfig

func Test_DockerLazyPull(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "LazyPull Docker Test Suite")
}

var _ = Describe("LazyPull Tests", Ordered, func() {

	Context("Setup Cluster", func() {
		It("should provision servers", func() {
			var err error
			config, err = tester.NewTestConfig(*k3sImage)
			Expect(err).NotTo(HaveOccurred())
			config.ServerYaml = "snapshotter: stargz"
			Expect(config.ProvisionServers(1)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server", "traefik"}, config.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})

	Context("Use Snapshot Container", func() {
		It("should apply local storage volume", func() {
			_, err := config.DeployWorkload("snapshot-test.yaml")
			Expect(err).NotTo(HaveOccurred(), "failed to apply volume test manifest")
		})
		It("should have the pod come up", func() {
			Eventually(func() (bool, error) {
				return tests.PodReady("stargz-snapshot-test", "default", config.KubeconfigFile)
			}, "30s", "5s").Should(BeTrue())
		})
		var topLayer string
		It("extracts the topmost layer of the container", func() {
			Eventually(func() (string, error) {
				var err error
				topLayer, err = getTopmostLayer(config.Servers[0].Name, "stargz-snapshot-test")
				topLayer = strings.TrimSpace(topLayer)
				return topLayer, err
			}, "30s", "5s").ShouldNot(BeEmpty())
			fmt.Println("Topmost layer: ", topLayer)
		})
		It("checks all layers are remote snapshots", func() {
			Expect(lookLayers(config.Servers[0].Name, topLayer)).To(Succeed())
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

func lookLayers(node, layer string) error {
	remoteSnapshotLabel := "containerd.io/snapshot/remote"
	layersNum := 0
	var err error
	for layersNum = 0; layersNum < 100; layersNum++ {
		// We use RunCommand instead of RunCmdOnNode because we pipe the output to jq
		cmd := fmt.Sprintf("docker exec -i %s ctr --namespace=k8s.io snapshot --snapshotter=stargz info %s | jq -r '.Parent'", node, layer)
		layer, err = tester.RunCommand(cmd)
		if err != nil {
			return fmt.Errorf("failed to get parent layer: %v", err)
		}
		layer = strings.TrimSpace(layer)
		// If the layer is null, we have reached the topmost layer
		if layer == "null" {
			break
		}
		cmd = fmt.Sprintf("docker exec -i %s ctr --namespace=k8s.io snapshots --snapshotter=stargz info %s | jq -r '.Labels.\"%s\"'", node, layer, remoteSnapshotLabel)
		label, err := tester.RunCommand(cmd)
		if err != nil {
			return fmt.Errorf("failed to get layer label: %v", err)
		}
		label = strings.TrimSpace(label)
		fmt.Printf("Checking layer %s : %s\n", layer, label)
		if label == "null" {
			return fmt.Errorf("layer %s isn't remote snapshot", layer)
		}
	}

	if layersNum == 0 {
		return fmt.Errorf("cannot get layers")
	} else if layersNum >= 100 {
		return fmt.Errorf("testing image contains too many layers > 100")
	}

	return nil
}

func getTopmostLayer(node, container string) (string, error) {
	var targetContainer string
	cmd := fmt.Sprintf("docker exec -i %s ctr --namespace=k8s.io c ls -q labels.\"io.kubernetes.container.name\"==\"%s\" | sed -n 1p", node, container)
	targetContainer, err := tester.RunCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get target container: %v", err)
	}
	targetContainer = strings.TrimSpace(targetContainer)
	fmt.Println("targetContainer: ", targetContainer)
	if targetContainer == "" {
		return "", fmt.Errorf("failed to get target container")
	}
	cmd = fmt.Sprintf("docker exec -i %s ctr --namespace=k8s.io c info %s | jq -r '.SnapshotKey'", node, targetContainer)
	layer, err := tester.RunCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get topmost layer: %v", err)
	}
	return strings.TrimSpace(layer), nil
}
