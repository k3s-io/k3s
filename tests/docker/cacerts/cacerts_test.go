package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	tester "github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k3sImage = flag.String("k3sImage", "", "The k3s image used to provision containers")
var config *tester.TestConfig
var testID string

func Test_DockerCACerts(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "CA Certs Docker Test Suite")
}

var _ = Describe("CA Certs Tests", Ordered, func() {

	Context("Setup Cluster", func() {
		// TODO determine if the below is still true
		// This test runs in docker mounting the docker socket,
		// so we can't directly mount files into the test containers. Instead we have to
		// run a dummy container with a volume, copy files into that volume, and then
		// share it with the other containers that need the file.
		It("should configure CA certs", func() {
			var err error
			config, err = tester.NewTestConfig(*k3sImage)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.MkdirAll(filepath.Join(config.TestDir, "pause"), 0755)).To(Succeed())

			testID = filepath.Base(config.TestDir)
			pauseName := fmt.Sprintf("k3s-pause-%s", strings.ToLower(testID))
			tlsMount := fmt.Sprintf("--mount type=volume,src=%s,dst=/var/lib/rancher/k3s/server/tls", pauseName)
			cmd := fmt.Sprintf("docker run -d --name %s --hostname %s %s rancher/mirrored-pause:3.6",
				pauseName, pauseName, tlsMount)
			_, err = tester.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			dataDir := filepath.Join(config.TestDir, "pause/k3s")
			cmd = fmt.Sprintf("DATA_DIR=%s ../../../contrib/util/generate-custom-ca-certs.sh", dataDir)
			_, err = tester.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = fmt.Sprintf("docker cp %s %s:/var/lib/rancher", dataDir, pauseName)
			_, err = tester.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Set SERVER_ARGS to include the custom CA certs
			os.Setenv("SERVER_DOCKER_ARGS", tlsMount)
		})

		It("should provision servers and agents", func() {
			Expect(config.ProvisionServers(1)).To(Succeed())
			Expect(config.ProvisionAgents(1)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server", "traefik"}, config.KubeconfigFile)
			}, "120s", "5s").Should(Succeed())
		})
	})

	Context("Verify Custom CA Certs", func() {
		It("should have custom CA certs", func() {
			// Add your custom CA certs verification logic here
			// Example: Check if the custom CA certs are present in the server container
			for _, server := range config.Servers {
				cmd := fmt.Sprintf("docker exec %s ls /var/lib/rancher/k3s/server/tls", server.Name)
				output, err := tester.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred(), "failed to list custom CA certs: %v", err)
				Expect(output).To(ContainSubstring("ca.crt"))
			}
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
		cmd := fmt.Sprintf("docker stop k3s-pause-%s", testID)
		_, err := tester.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred())
		cmd = fmt.Sprintf("docker rm k3s-pause-%s", testID)
		_, err = tester.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred())
		cmd = fmt.Sprintf("docker volume ls -q | grep -F %s | xargs -r docker volume rm -f", testID)
		_, err = tester.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred())
	}

})
