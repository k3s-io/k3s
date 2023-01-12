package snapshotrestore

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Valid nodeOS:
// generic/ubuntu2004, generic/centos7, generic/rocky8,
// opensuse/Leap-15.3.x86_64, dweomer/microos.amd64

var nodeOS = flag.String("nodeOS", "generic/ubuntu2004", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var hardened = flag.Bool("hardened", false, "true or false")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
// E2E_EXTERNAL_DB: mysql, postgres, etcd (default: etcd)
// E2E_RELEASE_VERSION=v1.23.1+k3s2 (default: latest commit from master)

func Test_E2ESnapshotRestore(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "SnapshotRestore Test Suite", suiteConfig, reporterConfig)
}

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
	snapshotname    string
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Create", Ordered, func() {
	Context("Cluster :", func() {
		It("Starts up with no issues", func() {
			var err error
			if *local {
				serverNodeNames, agentNodeNames, err = e2e.CreateLocalCluster(*nodeOS, *serverCount, *agentCount)
			} else {
				serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
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
			}, "420s", "5s").Should(Succeed())
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

		It("Verifies test workload before snapshot is created", func() {
			res, err := e2e.DeployWorkload("clusterip.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed: "+res)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should((ContainSubstring("test-clusterip")), "failed cmd: "+cmd+" result: "+res)
			}, "240s", "5s").Should(Succeed())
		})

		It("Verifies Snapshot is created", func() {
			Eventually(func(g Gomega) {
				cmd := "sudo k3s etcd-snapshot"
				_, err := e2e.RunCmdOnNode(cmd, "server-0")
				g.Expect(err).NotTo(HaveOccurred())
				cmd = "sudo ls /var/lib/rancher/k3s/server/db/snapshots/"
				snapshotname, err = e2e.RunCmdOnNode(cmd, "server-0")
				fmt.Println("Snapshot Name", snapshotname)
				g.Expect(snapshotname).Should(ContainSubstring("on-demand-server-0"))
			}, "420s", "10s").Should(Succeed())
		})

		It("Verifies another test workload after snapshot is created", func() {
			_, err := e2e.DeployWorkload("nodeport.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")
			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				res, err := e2e.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")
			}, "240s", "5s").Should(Succeed())
		})

		It("Verifies snapshot is restored successfully and validates only test workload1 is present", func() {
			//Stop k3s on all nodes
			for _, nodeName := range serverNodeNames {
				cmd := "sudo systemctl stop k3s"
				_, err := e2e.RunCmdOnNode(cmd, nodeName)
				Expect(err).NotTo(HaveOccurred())
			}
			//Restores from snapshot on server-0
			for _, nodeName := range serverNodeNames {
				if nodeName == "server-0" {
					cmd := "sudo k3s server --cluster-init --cluster-reset --cluster-reset-restore-path=/var/lib/rancher/k3s/server/db/snapshots/" + snapshotname
					res, err := e2e.RunCmdOnNode(cmd, nodeName)
					Expect(err).NotTo(HaveOccurred())
					Expect(res).Should(ContainSubstring("Managed etcd cluster membership has been reset, restart without --cluster-reset flag now"))

					cmd = "sudo systemctl start k3s"
					_, err = e2e.RunCmdOnNode(cmd, nodeName)
					Expect(err).NotTo(HaveOccurred())
				}
			}
			//Verifies node is up and pods running
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())
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
			//Verifies test workload1 is present
			//Verifies test workload2 is not present
			cmd := "kubectl get pods --kubeconfig=" + kubeConfigFile
			res, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring("test-clusterip"))
			Expect(res).ShouldNot(ContainSubstring("test-nodeport"))
		})
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
