package snapshotrestore

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
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var hardened = flag.Bool("hardened", false, "true or false")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")

// Environment Variables Info:
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

var _ = Describe("Verify snapshots and cluster restores work", Ordered, func() {
	Context("Cluster creates snapshots and workloads:", func() {
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
				cmd := "k3s etcd-snapshot save"
				_, err := e2e.RunCmdOnNode(cmd, "server-0")
				g.Expect(err).NotTo(HaveOccurred())
				cmd = "ls /var/lib/rancher/k3s/server/db/snapshots/"
				snapshotname, err = e2e.RunCmdOnNode(cmd, "server-0")
				g.Expect(err).NotTo(HaveOccurred())
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

	})

	Context("Cluster is reset normally", func() {
		It("Resets the cluster", func() {
			for _, nodeName := range serverNodeNames {
				cmd := "systemctl stop k3s"
				Expect(e2e.RunCmdOnNode(cmd, nodeName)).Error().NotTo(HaveOccurred())
				if nodeName != serverNodeNames[0] {
					cmd = "k3s-killall.sh"
					Expect(e2e.RunCmdOnNode(cmd, nodeName)).Error().NotTo(HaveOccurred())
				}
			}

			cmd := "k3s server --cluster-reset"
			res, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring("Managed etcd cluster membership has been reset, restart without --cluster-reset flag now"))

			cmd = "systemctl start k3s"
			Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Error().NotTo(HaveOccurred())
		})

		It("Resets non bootstrap nodes", func() {
			for _, nodeName := range serverNodeNames {
				if nodeName != serverNodeNames[0] {
					cmd := "k3s server --cluster-reset"
					response, err := e2e.RunCmdOnNode(cmd, nodeName)
					Expect(err).NotTo(HaveOccurred())
					Expect(response).Should(ContainSubstring("Managed etcd cluster membership has been reset, restart without --cluster-reset flag now"))
				}
			}
		})

		It("Checks that other servers are not ready", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					if strings.Contains(node.Name, serverNodeNames[0]) || strings.Contains(node.Name, "agent-") {
						g.Expect(node.Status).Should(Equal("Ready"))
					} else {
						g.Expect(node.Status).Should(Equal("NotReady"))
					}
				}
			}, "240s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(kubeConfigFile, true)
		})

		It("Rejoins other servers to cluster", func() {
			// We must remove the db directory on the other servers before restarting k3s
			// otherwise the nodes may join the old cluster
			for _, nodeName := range serverNodeNames[1:] {
				cmd := "rm -rf /var/lib/rancher/k3s/server/db"
				Expect(e2e.RunCmdOnNode(cmd, nodeName)).Error().NotTo(HaveOccurred())
			}

			for _, nodeName := range serverNodeNames[1:] {
				cmd := "systemctl start k3s"
				Expect(e2e.RunCmdOnNode(cmd, nodeName)).Error().NotTo(HaveOccurred())
				time.Sleep(20 * time.Second) //Stagger the restarts for etcd leaners
			}
		})

		It("Checks that all nodes and pods are ready", func() {
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					nodeJournal, _ := e2e.GetJournalLogs(node.Name)
					g.Expect(node.Status).Should(Equal("Ready"), nodeJournal)
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
			}, "420s", "5s").Should(Succeed())
		})
		It("Verifies that workload1 and workload1 exist", func() {
			cmd := "kubectl get pods --kubeconfig=" + kubeConfigFile
			res, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring("test-clusterip"))
			Expect(res).Should(ContainSubstring("test-nodeport"))
		})

	})

	Context("Cluster restores from snapshot", func() {
		It("Restores the snapshot", func() {
			//Stop k3s on all nodes
			for _, nodeName := range serverNodeNames {
				cmd := "systemctl stop k3s"
				Expect(e2e.RunCmdOnNode(cmd, nodeName)).Error().NotTo(HaveOccurred())
				if nodeName != serverNodeNames[0] {
					cmd = "k3s-killall.sh"
					Expect(e2e.RunCmdOnNode(cmd, nodeName)).Error().NotTo(HaveOccurred())
				}
			}
			//Restores from snapshot on server-0
			cmd := "k3s server --cluster-init --cluster-reset --cluster-reset-restore-path=/var/lib/rancher/k3s/server/db/snapshots/" + snapshotname
			res, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring("Managed etcd cluster membership has been reset, restart without --cluster-reset flag now"))

			cmd = "systemctl start k3s"
			Expect(e2e.RunCmdOnNode(cmd, serverNodeNames[0])).Error().NotTo(HaveOccurred())

		})

		It("Checks that other servers are not ready", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					if strings.Contains(node.Name, serverNodeNames[0]) || strings.Contains(node.Name, "agent-") {
						g.Expect(node.Status).Should(Equal("Ready"))
					} else {
						g.Expect(node.Status).Should(Equal("NotReady"))
					}
				}
			}, "240s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(kubeConfigFile, true)
		})

		It("Rejoins other servers to cluster", func() {
			// We must remove the db directory on the other servers before restarting k3s
			// otherwise the nodes may join the old cluster
			for _, nodeName := range serverNodeNames[1:] {
				cmd := "rm -rf /var/lib/rancher/k3s/server/db"
				Expect(e2e.RunCmdOnNode(cmd, nodeName)).Error().NotTo(HaveOccurred())
			}

			for _, nodeName := range serverNodeNames[1:] {
				cmd := "systemctl start k3s"
				Expect(e2e.RunCmdOnNode(cmd, nodeName)).Error().NotTo(HaveOccurred())
			}
		})

		It("Checks that all nodes and pods are ready", func() {
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
		})

		It("Verifies that workload1 exists and workload2 does not", func() {
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
	if !failed {
		Expect(e2e.GetCoverageReport(append(serverNodeNames, agentNodeNames...))).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
