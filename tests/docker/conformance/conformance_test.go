package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/k3s-io/k3s/tests"
	tester "github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k3sImage = flag.String("k3sImage", "", "The k3s image used to provision containers")
var db = flag.String("db", "", "The database to use for the tests (sqlite, etcd, mysql, postgres)")
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
			config.DBType = *db
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
			cmd := fmt.Sprintf("%s --focus=\"Conformance\" --skip=\"Serial|Flaky\" -v 2 -p %d --kubeconfig %s",
				filepath.Join(config.TestDir, "hydrophone"),
				runtime.NumCPU()/2,
				config.KubeconfigFile)
			By("Hydrophone: " + cmd)
			hc, err := StartCmd(cmd)
			Expect(err).NotTo(HaveOccurred())
			// Periodically check the number of tests that have run, since the hydrophone output does not support a progress status
			// Taken from https://github.com/kubernetes-sigs/hydrophone/issues/223#issuecomment-2547174722
			go func() {
				cmd := fmt.Sprintf("kubectl exec -n=conformance e2e-conformance-test -c output-container --kubeconfig=%s -- cat /tmp/results/e2e.log | grep -o \"•\" | wc -l",
					config.KubeconfigFile)
				for i := 1; ; i++ {
					time.Sleep(120 * time.Second)
					if hc.ProcessState != nil {
						break
					}
					res, _ := tester.RunCommand(cmd)
					res = strings.TrimSpace(res)
					fmt.Printf("Status Report %d: %s tests complete\n", i, res)
				}
			}()
			Expect(hc.Wait()).To(Succeed())
		})
		It("should run serial conformance tests", func() {
			if !*serial {
				Skip("Skipping serial conformance tests")
			}
			cmd := fmt.Sprintf("%s --focus=\"Serial\" --skip=\"Flaky\"  -v 2 --kubeconfig %s",
				filepath.Join(config.TestDir, "hydrophone"),
				config.KubeconfigFile)
			By("Hydrophone: " + cmd)
			hc, err := StartCmd(cmd)
			Expect(err).NotTo(HaveOccurred())
			go func() {
				cmd := fmt.Sprintf("kubectl exec -n=conformance e2e-conformance-test -c output-container --kubeconfig=%s -- cat /tmp/results/e2e.log | grep -o \"•\" | wc -l",
					config.KubeconfigFile)
				for i := 1; ; i++ {
					time.Sleep(120 * time.Second)
					if hc.ProcessState != nil {
						break
					}
					res, _ := tester.RunCommand(cmd)
					res = strings.TrimSpace(res)
					fmt.Printf("Status Report %d: %s tests complete\n", i, res)
				}
			}()
			Expect(hc.Wait()).To(Succeed())
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

// StartCmd starts a command and pipes its output to
// the ginkgo Writr, with the expectation to poll the progress of the command
func StartCmd(cmd string) (*exec.Cmd, error) {
	c := exec.Command("sh", "-c", cmd)
	c.Stdout = GinkgoWriter
	c.Stderr = GinkgoWriter
	if err := c.Start(); err != nil {
		return c, err
	}
	return c, nil
}
