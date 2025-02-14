package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	tester "github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k3sImage = flag.String("k3sImage", "", "The image used to provision containers")
var config *tester.TestConfig

func Test_DockerBasic(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Basic Docker Test Suite")
}

var _ = Describe("Basic Tests", Ordered, func() {

	Context("Setup Cluster", func() {
		It("should provision servers and agents", func() {
			var err error
			config, err = tester.NewTestConfig(*k3sImage)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ProvisionServers(1)).To(Succeed())
			Expect(config.ProvisionAgents(1)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server", "traefik"}, config.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})

	Context("Use Local Storage Volume", func() {
		It("should apply local storage volume", func() {
			_, err := config.DeployWorkload("volume-test.yaml")
			Expect(err).NotTo(HaveOccurred(), "failed to apply volume test manifest")
		})
		It("should validate local storage volume", func() {
			Eventually(func() (bool, error) {
				return tests.PodReady("volume-test", "kube-system", config.KubeconfigFile)
			}, "20s", "5s").Should(BeTrue())
		})
	})

	Context("Verify Binaries and Images", func() {
		It("has valid bundled binaries", func() {
			for _, server := range config.Servers {
				Expect(tester.VerifyValidVersion(server, "kubectl")).To(Succeed())
				Expect(tester.VerifyValidVersion(server, "ctr")).To(Succeed())
				Expect(tester.VerifyValidVersion(server, "crictl")).To(Succeed())
			}
		})
		It("has valid airgap images", func() {
			Expect(config).To(Not(BeNil()))
			err := VerifyAirgapImages(config)
			Expect(err).NotTo(HaveOccurred())
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

// VerifyAirgapImages checks for changes in the airgap image list
func VerifyAirgapImages(config *tester.TestConfig) error {
	// This file is generated during the build packaging step
	const airgapImageList = "../../../scripts/airgap/image-list.txt"

	// Use a map to automatically handle duplicates
	imageSet := make(map[string]struct{})

	// Collect all images from nodes
	for _, node := range config.GetNodeNames() {
		cmd := fmt.Sprintf("docker exec %s crictl images -o json | jq -r '.images[].repoTags[0] | select(. != null)'", node)
		output, err := tester.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed to execute crictl and jq: %v", err)

		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if line != "" {
				imageSet[line] = struct{}{}
			}
		}
	}

	// Convert map keys to slice
	uniqueImages := make([]string, 0, len(imageSet))
	for image := range imageSet {
		uniqueImages = append(uniqueImages, image)
	}

	existing, err := os.ReadFile(airgapImageList)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read airgap list file: %v", err)
	}

	// Sorting doesn't matter with ConsistOf
	existingImages := strings.Split(strings.TrimSpace(string(existing)), "\n")
	Expect(existingImages).To(ConsistOf(uniqueImages))
	return nil
}
