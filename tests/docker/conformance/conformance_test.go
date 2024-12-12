package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/k3s-io/k3s/tests"
	tester "github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k3sImage = flag.String("k3sImage", "", "The k3s image used to provision containers")
var serial = flag.Bool("serial", false, "Run the Serial Conformance Tests")
var config *tester.TestConfig

func Test_DockerConformance(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Conformance Docker Test Suite")
}

var _ = Describe("Conformance Tests", Ordered, func() {

	Context("Setup Cluster", func() {
		It("should provision servers and agents", func() {
			var err error
			config, err = tester.NewTestConfig(*k3sImage)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ProvisionServers(1)).To(Succeed())
			Expect(config.ProvisionAgents(1)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(config.KubeconfigFile)
			}, "90s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})
	Context("Run Hydrophone Conformance tests", func() {
		It("should download hydrophone", func() {
			hydrophoneVersion := "v0.6.0"
			hydrophoneArch := runtime.GOARCH
			if hydrophoneArch == "amd64" {
				hydrophoneArch = "x86_64"
			}
			hydrophoneURL := fmt.Sprintf("https://github.com/kubernetes-sigs/hydrophone/releases/download/%s/hydrophone_Linux_%s.tar.gz",
				hydrophoneVersion, hydrophoneArch)
			cmd := fmt.Sprintf("curl -L %s | tar -xzf - -C %s", hydrophoneURL, config.TestDir)
			_, err := tester.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Chmod(filepath.Join(config.TestDir, "hydrophone"), 0755)).To(Succeed())
		})
		// Takes about 15min to run, so expect nothing to happen for a while
		It("should run parallel conformance tests", func() {
			if *serial {
				Skip("Skipping parallel conformance tests")
			}
			cmd := fmt.Sprintf("%s --focus=\"Conformance\" --skip=\"Serial|Flaky\" -p %d --extra-ginkgo-args=\"%s\" --kubeconfig %s",
				filepath.Join(config.TestDir, "hydrophone"),
				runtime.NumCPU()/2,
				"--poll-progress-after=60s,--poll-progress-interval=30s",
				config.KubeconfigFile)
			By("Hydrophone: " + cmd)
			res, err := tester.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), res)
		})
		It("should run serial conformance tests", func() {
			if !*serial {
				Skip("Skipping serial conformance tests")
			}
			cmd := fmt.Sprintf("%s -o %s --focus=\"Serial\" --skip=\"Flaky\"  --extra-ginkgo-args=\"%s\" --kubeconfig %s",
				filepath.Join(config.TestDir, "hydrophone"),
				filepath.Join(config.TestDir, "logs"),
				"--poll-progress-after=60s,--poll-progress-interval=30s",
				config.KubeconfigFile)
			By("Hydrophone: " + cmd)
			res, err := tester.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), res)
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
