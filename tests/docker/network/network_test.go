package network

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k3sImage = flag.String("k3sImage", "rancher/k3s:latest", "The image used to provision containers")

func Test_DockerNetwork(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Network Docker Test Suite")
}

var _ = Describe("Network Tests", Ordered, func() {
	Context("Verify startup with temporarily missing default route", func() {
		var containerName string

		It("Starts K3s with default route removed", func() {
			containerName = "k3s-network-test"
			
			// Get path to locally compiled binary directory
			pwd, _ := os.Getwd()
			artifactDir := filepath.Join(pwd, "..", "..", "..", "dist", "artifacts")
			
			// Clean up any lingering container from past test runs
			tests.RunCommand(fmt.Sprintf("docker rm -f %s", containerName))

			// Boot K3s container, removing the default route before K3s starts.
			dRun := strings.Join([]string{"docker run -d",
				"--name", containerName,
				"--hostname", containerName,
				"--privileged",
				"-e K3S_DEBUG=true",
				fmt.Sprintf("--mount type=bind,source=%s,target=/opt/artifacts", artifactDir),
				"--entrypoint sh",
				*k3sImage,
				"-c",
				`"ip route del default && /opt/artifacts/k3s server"`}, " ")

			out, err := tests.RunCommand(dRun)
			Expect(err).NotTo(HaveOccurred(), "failed to run server container: %s", out)
		})

		It("Verifies the retry loop is active and waiting", func() {
			// Instead of crashing instantly, it should hold in your retry loop
			Eventually(func() (string, error) {
				return tests.RunCommand(fmt.Sprintf("docker logs %s", containerName))
			}, "20s", "2s").Should(ContainSubstring("Waiting for default network route to become available..."))
		})

		It("Restores the network dynamically", func() {
			// Find the default gateway for this container from docker inspect
			inspectCmd := fmt.Sprintf("docker inspect --format '{{range .NetworkSettings.Networks}}{{.Gateway}}{{end}}' %s", containerName)
			gateway, err := tests.RunCommand(inspectCmd)
			Expect(err).NotTo(HaveOccurred(), "failed to get gateway: %s", gateway)
			gateway = strings.Trim(strings.TrimSpace(gateway), "'")

			// Restore the default route dynamically while the container is running
			addRouteCmd := fmt.Sprintf("docker exec %s ip route add default via %s dev eth0", containerName, gateway)
			out, err := tests.RunCommand(addRouteCmd)
			Expect(err).NotTo(HaveOccurred(), "failed to restore default route: %s", out)
		})

		It("Verifies K3s recovers and successfully starts up", func() {
			// We query the nodes directly from within the container using our local binary
			Eventually(func() (string, error) {
				cmd := fmt.Sprintf("docker exec %s /opt/artifacts/k3s kubectl get nodes", containerName)
				return tests.RunCommand(cmd)
			}, "120s", "5s").Should(ContainSubstring("Ready"))
		})

		It("Cleans up the container", func() {
			_, err := tests.RunCommand(fmt.Sprintf("docker rm -f %s", containerName))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
