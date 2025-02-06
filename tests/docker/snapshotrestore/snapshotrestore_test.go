package snapshotrestore

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	tester "github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/set"
)

var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var config *tester.TestConfig
var snapshotname string

func Test_DockerSnapshotRestore(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "SnapshotRestore Test Suite", suiteConfig, reporterConfig)
}

var _ = Describe("Verify snapshots and cluster restores work", Ordered, func() {
	Context("Setup Cluster", func() {
		It("should provision servers and agents", func() {
			var err error
			config, err = tester.NewTestConfig("rancher/systemd-node")
			Expect(err).NotTo(HaveOccurred())
			Expect(config.ProvisionServers(*serverCount)).To(Succeed())
			Expect(config.ProvisionAgents(*agentCount)).To(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(config.KubeconfigFile)
			}, "60s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})
	Context("Cluster creates snapshots and workloads:", func() {
		It("Verifies test workload before snapshot is created", func() {
			res, err := config.DeployWorkload("clusterip.yaml")
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed: "+res)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + config.KubeconfigFile
				res, err := tester.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should((ContainSubstring("test-clusterip")), "failed cmd: %q result: %s", cmd, res)
			}, "240s", "5s").Should(Succeed())
		})

		It("Verifies Snapshot is created", func() {
			Eventually(func(g Gomega) {
				_, err := config.Servers[0].RunCmdOnNode("k3s etcd-snapshot save")
				g.Expect(err).NotTo(HaveOccurred())
				cmd := "ls /var/lib/rancher/k3s/server/db/snapshots/"
				snapshotname, err = config.Servers[0].RunCmdOnNode(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				fmt.Println("Snapshot Name", snapshotname)
				g.Expect(snapshotname).Should(ContainSubstring("on-demand-server-0"))
			}, "240s", "10s").Should(Succeed())
		})

		It("Verifies another test workload after snapshot is created", func() {
			res, err := config.DeployWorkload("nodeport.yaml")
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed: "+res)
			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + config.KubeconfigFile
				res, err := tester.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")
			}, "240s", "5s").Should(Succeed())
		})

	})

	Context("Cluster restores from snapshot", func() {
		It("Restores the snapshot", func() {
			//Stop k3s on all servers
			for _, server := range config.Servers {
				cmd := "systemctl stop k3s"
				Expect(server.RunCmdOnNode(cmd)).Error().NotTo(HaveOccurred())
				if server != config.Servers[0] {
					cmd = "k3s-killall.sh"
					Expect(server.RunCmdOnNode(cmd)).Error().NotTo(HaveOccurred())
				}
			}
			//Restores from snapshot on server-0
			cmd := "k3s server --cluster-init --cluster-reset --cluster-reset-restore-path=/var/lib/rancher/k3s/server/db/snapshots/" + snapshotname
			res, err := config.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring("Managed etcd cluster membership has been reset, restart without --cluster-reset flag now"))

			cmd = "systemctl start k3s"
			Expect(config.Servers[0].RunCmdOnNode(cmd)).Error().NotTo(HaveOccurred())

		})

		It("Checks that other servers are not ready", func() {
			By("Fetching node status")
			var readyNodeNames []string
			var notReadyNodeNames []string
			Eventually(func(g Gomega) {
				readyNodeNames = []string{config.Servers[0].Name}
				for _, agent := range config.Agents {
					readyNodeNames = append(readyNodeNames, agent.Name)
				}
				for _, server := range config.Servers[1:] {
					notReadyNodeNames = append(notReadyNodeNames, server.Name)
				}
				g.Expect(CheckNodeStatus(config.KubeconfigFile, readyNodeNames, notReadyNodeNames)).To(Succeed())
			}, "240s", "5s").Should(Succeed())
		})

		It("Rejoins other servers to cluster", func() {
			// We must remove the db directory on the other servers before restarting k3s
			// otherwise the nodes may join the old cluster
			for _, server := range config.Servers[1:] {
				cmd := "rm -rf /var/lib/rancher/k3s/server/db"
				Expect(server.RunCmdOnNode(cmd)).Error().NotTo(HaveOccurred())
			}

			for _, server := range config.Servers[1:] {
				cmd := "systemctl start k3s"
				Expect(server.RunCmdOnNode(cmd)).Error().NotTo(HaveOccurred())
			}
		})

		It("Checks that all nodes and pods are ready", func() {
			By("Fetching node status")
			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "60s", "5s").Should(Succeed())

			By("Fetching Pods status")
			Eventually(func(g Gomega) {
				pods, err := tests.ParsePods(config.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(string(pod.Status.Phase)).Should(Equal("Succeeded"), pod.Name)
					} else {
						g.Expect(string(pod.Status.Phase)).Should(Equal("Running"), pod.Name)
					}
				}
			}, "120s", "5s").Should(Succeed())
		})

		It("Verifies that workload1 exists and workload2 does not", func() {
			cmd := "kubectl get pods --kubeconfig=" + config.KubeconfigFile
			res, err := tester.RunCommand(cmd)
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
	if *ci || (config != nil && !failed) {
		Expect(config.Cleanup()).To(Succeed())
	}
})

// Checks if nodes match the expected status
// We use kubectl directly, because getting a NotReady node status from the API is not easy
func CheckNodeStatus(kubeconfigFile string, readyNodes, notReadyNodes []string) error {
	readyNodesSet := set.New(readyNodes...)
	notReadyNodesSet := set.New(notReadyNodes...)
	foundReadyNodes := make(set.Set[string], 0)
	foundNotReadyNodes := make(set.Set[string], 0)

	cmd := "kubectl get nodes --no-headers --kubeconfig=" + kubeconfigFile
	res, err := tester.RunCommand(cmd)
	if err != nil {
		return err
	}
	// extract the node status from the 2nd column of kubectl output
	for _, line := range strings.Split(res, "\n") {
		if strings.Contains(line, "k3s-test") {
			// Line for some reason needs to be split twice
			split := strings.Fields(line)
			status := strings.TrimSpace(split[1])
			if status == "NotReady" {
				foundNotReadyNodes.Insert(split[0])
			} else if status == "Ready" {
				foundReadyNodes.Insert(split[0])
			}
		}
	}
	if !foundReadyNodes.Equal(readyNodesSet) {
		return fmt.Errorf("expected ready nodes %v, found %v", readyNodesSet, foundReadyNodes)
	}
	if !foundNotReadyNodes.Equal(notReadyNodesSet) {
		return fmt.Errorf("expected not ready nodes %v, found %v", notReadyNodesSet, foundNotReadyNodes)
	}
	return nil
}
