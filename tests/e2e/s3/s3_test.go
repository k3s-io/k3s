package s3

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS:
// bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
// eurolinux-vagrant/rocky-8, eurolinux-vagrant/rocky-9,
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 (default: latest commit from master)
// E2E_REGISTRY: true/false (default: false)

func Test_E2ES3(t *testing.T) {
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
				serverNodeNames, agentNodeNames, err = e2e.CreateLocalCluster(*nodeOS, 1, 0)
			} else {
				serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, 1, 0)
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

		It("ensures s3 mock is working", func() {
			res, err := e2e.RunCmdOnNode("docker ps -a | grep mock\n", serverNodeNames[0])
			fmt.Println(res)
			Expect(err).NotTo(HaveOccurred())
		})
		It("save s3 snapshot using CLI", func() {
			res, err := e2e.RunCmdOnNode("k3s etcd-snapshot save "+
				"--etcd-s3-insecure=true "+
				"--etcd-s3-bucket=test-bucket "+
				"--etcd-s3-folder=test-folder "+
				"--etcd-s3-endpoint=localhost:9090 "+
				"--etcd-s3-skip-ssl-verify=true "+
				"--etcd-s3-access-key=test ",
				serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(ContainSubstring("Snapshot on-demand-server-0"))
		})
		It("creates s3 config secret", func() {
			res, err := e2e.RunCmdOnNode("k3s kubectl create secret generic k3s-etcd-s3-config --namespace=kube-system "+
				"--from-literal=etcd-s3-insecure=true "+
				"--from-literal=etcd-s3-bucket=test-bucket "+
				"--from-literal=etcd-s3-folder=test-folder "+
				"--from-literal=etcd-s3-endpoint=localhost:9090 "+
				"--from-literal=etcd-s3-skip-ssl-verify=true "+
				"--from-literal=etcd-s3-access-key=test ",
				serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(ContainSubstring("secret/k3s-etcd-s3-config created"))
		})
		It("save s3 snapshot using secret", func() {
			res, err := e2e.RunCmdOnNode("k3s etcd-snapshot save", serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(ContainSubstring("Snapshot on-demand-server-0"))
		})
		It("lists saved s3 snapshot", func() {
			res, err := e2e.RunCmdOnNode("k3s etcd-snapshot list", serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(ContainSubstring("file:///var/lib/rancher/k3s/server/db/snapshots/on-demand-server-0"))
			Expect(res).To(ContainSubstring("s3://test-bucket/test-folder/on-demand-server-0"))
		})
		It("save 3 more s3 snapshots", func() {
			for _, i := range []string{"1", "2", "3"} {
				res, err := e2e.RunCmdOnNode("k3s etcd-snapshot save --name special-"+i, serverNodeNames[0])
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(ContainSubstring("Snapshot special-" + i + "-server-0"))
			}
		})
		It("lists saved s3 snapshot", func() {
			res, err := e2e.RunCmdOnNode("k3s etcd-snapshot list", serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(ContainSubstring("s3://test-bucket/test-folder/on-demand-server-0"))
			Expect(res).To(ContainSubstring("s3://test-bucket/test-folder/special-1-server-0"))
			Expect(res).To(ContainSubstring("s3://test-bucket/test-folder/special-2-server-0"))
			Expect(res).To(ContainSubstring("s3://test-bucket/test-folder/special-3-server-0"))
		})
		It("delete first on-demand s3 snapshot", func() {
			_, err := e2e.RunCmdOnNode("sudo k3s etcd-snapshot ls >> ./snapshotname.txt", serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			snapshotName, err := e2e.RunCmdOnNode("grep -Eo 'on-demand-server-0-([0-9]+)' ./snapshotname.txt | head -1", serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			res, err := e2e.RunCmdOnNode("sudo k3s etcd-snapshot delete "+snapshotName, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(ContainSubstring("Snapshot " + strings.TrimSpace(snapshotName) + " deleted"))
		})
		It("prunes s3 snapshots", func() {
			_, err := e2e.RunCmdOnNode("k3s etcd-snapshot save", serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			_, err = e2e.RunCmdOnNode("k3s etcd-snapshot save", serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			res, err := e2e.RunCmdOnNode("k3s etcd-snapshot prune", serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			// There should now be 4 on-demand snapshots - 2 local, and 2 on s3
			res, err = e2e.RunCmdOnNode("k3s etcd-snapshot ls 2>/dev/null | grep on-demand | wc -l", serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(res)).To(Equal("4"))
		})
		It("ensure snapshots retention is working in s3 and local", func() {
			// Wait until the retention works with 3 minutes
			fmt.Printf("\nWaiting 3 minutes until retention works\n")
			time.Sleep(3 * time.Minute)
			res, err := e2e.RunCmdOnNode("k3s etcd-snapshot ls 2>/dev/null | grep etcd-snapshot | wc -l", serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(res)).To(Equal("4"))
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
