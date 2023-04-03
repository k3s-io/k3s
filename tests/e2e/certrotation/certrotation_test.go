package secretsencryption

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS: generic/ubuntu2004, opensuse/Leap-15.3.x86_64
var nodeOS = flag.String("nodeOS", "generic/ubuntu2204", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 or nil for latest commit from master

func Test_E2ECustomCARotation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Custom Certificate Rotation Test Suite", suiteConfig, reporterConfig)
}

var (
	kubeConfigFile  string
	agentNodeNames  []string
	serverNodeNames []string
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Custom CA Rotation", Ordered, func() {
	Context("Custom CA is rotated:", func() {
		It("Starts up with no issues", func() {
			var err error
			serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			fmt.Println("CLUSTER CONFIG")
			fmt.Println("OS:", *nodeOS)
			fmt.Println("Server Nodes:", serverNodeNames)
			fmt.Println("Agent Nodes:", agentNodeNames)
			kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
		})

		It("Verifies Certificate Rotation", func() {
			const grepCert = "sudo ls -lt /var/lib/rancher/k3s/server/ | grep tls"
			//const expectResult = "client-ca.crt,client-ca.key,client-ca.nochain.crt,dynamic-cert.json,peer-ca.crt,peer-ca.key,server-ca.crt,server-ca.key,request-header-ca.crt,request-header-ca.key,server-ca.crt,server-ca.key,server-ca.nochain.crt,service.current.key,service.key,apiserver-loopback-client__.crt,apiserver-loopback-client__.key,"

			var expectResult = []string{"client-ca.crt", "client-ca.key", "client-ca.nochain.crt", "client-ca.pem",
				"dynamic-cert.json", "peer-ca.crt", "peer-ca.key", "peer-ca.pem", "server-ca.crt", "server-ca.key", "server-ca.pem",
				"intermediate-ca.crt", "intermediate-ca.key",
				"intermediate-ca.pem", "request-header-ca.crt", "request-header-ca.key",
				"request-header-ca.pem", "root-ca.crt", "root-ca.key", "root-ca.pem", "server-ca.crt", "server-ca.key",
				"server-ca.nochain.crt", "server-ca.pem", "service.current.key", "service.key", "apiserver-loopback-client__.crt",
				"apiserver-loopback-client__.key", "",
			}

			var finalResult string
			var finalErr error
			errStop := e2e.StopCluster(serverNodeNames)
			Expect(errStop).NotTo(HaveOccurred(), "Server not stop correctly")
			errRotate := e2e.RotateCertificate(serverNodeNames)
			Expect(errRotate).NotTo(HaveOccurred(), "Certificate not rotate correctly")
			errStart := e2e.StartCluster(serverNodeNames)
			Expect(errStart).NotTo(HaveOccurred(), "Server not start correctly")

			for _, nodeName := range serverNodeNames {
				grCert, errGrep := e2e.RunCmdOnNode(grepCert, nodeName)
				Expect(errGrep).NotTo(HaveOccurred(), "Certificate not created correctly")
				re := regexp.MustCompile("tls-[0-9]+")
				tls := re.FindAllString(grCert, -1)[0]
				final := fmt.Sprintf("sudo diff -sr /var/lib/rancher/k3s/server/tls/ /var/lib/rancher/k3s/server/%s/ | grep -i identical | cut -f4 -d ' ' | xargs basename -a \n", tls)
				finalResult, finalErr = e2e.RunCmdOnNode(final, nodeName)
				Expect(finalErr).NotTo(HaveOccurred(), "Final Certification does not created correctly")
			}
			if len(agentNodeNames) > 0 {
				errRestartAgent := e2e.RestartCluster(agentNodeNames)
				Expect(errRestartAgent).NotTo(HaveOccurred(), "Restart Agent not happened correctly")
			}
			Eventually(func(g Gomega) {
				finalCert := strings.Replace(finalResult, "\n", ",", -1)
				finalCertArray := strings.Split(finalCert, ",")
				fmt.Println("FINAL ESTEBAN ARRAY", finalCertArray)
				fmt.Println("FINAL ESTEBAN expectResult", expectResult)
				Expect((finalCertArray)).Should((Equal(expectResult)), "Final certification does not match the expected results")
			}, "620s", "5s").Should(Succeed())

		})

		/*It("Generates New CA Certificates", func() {
			cmds := []string{
				"sudo mkdir -p /opt/rancher/k3s/server",
				"sudo cp -r /var/lib/rancher/k3s/server/tls /opt/rancher/k3s/server",
				"sudo DATA_DIR=/opt/rancher/k3s /tmp/generate-custom-ca-certs.sh",
			}
			for _, cmd := range cmds {
				_, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("Rotates CA Certificates", func() {
			cmd := "sudo k3s certificate rotate-ca --path=/opt/rancher/k3s/server"
			_, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
		})

		It("Restarts K3s servers", func() {
			Expect(e2e.RestartCluster(serverNodeNames)).To(Succeed())
		})

		It("Restarts K3s agents", func() {
			Expect(e2e.RestartCluster(agentNodeNames)).To(Succeed())
		})

		It("Checks node and pod status", func() {
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())

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
			}, "420s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(kubeConfigFile, true)
		})*/
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed && !*ci {
		fmt.Println("FAILED!")
	} else {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
