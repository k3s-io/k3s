package snapshotrestore

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/set"
)

var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")

const defaultSnapshotPath = "/var/lib/rancher/k3s/server/db/snapshots/"

func Test_DockerSnapshotRestore(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "SnapshotRestore Test Suite", suiteConfig, reporterConfig)
}

type snapshotOptions struct {
	path     string
	compress bool
	s3       bool
}

var _ = DescribeTableSubtree("Verify snapshots and cluster restores work", Ordered, func(opts snapshotOptions) {
	var config *docker.TestConfig
	var snapshotname string
	var failed bool

	Context("Setup Cluster", func() {
		It("should start s3 mock", func() {
			_, err := tests.RunCommand("docker run --name s3mock -p 9090:9090 -p 9191:9191 -d -e COM_ADOBE_TESTING_S3MOCK_STORE_INITIAL_BUCKETS=test-bucket -e debug=true -t mirror.gcr.io/adobe/s3mock:5.0.0")
			Expect(err).NotTo(HaveOccurred())
		})
		It("should provision servers and agents", func() {
			var err error
			config, err = docker.NewTestConfig(GinkgoTB(), "rancher/systemd-node")
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
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running"
				res, err := tests.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should((ContainSubstring("test-clusterip")), "failed cmd: %q result: %s", cmd, res)
			}, "240s", "5s").Should(Succeed())
		})

		It("Verifies Snapshot is created", func() {
			Eventually(func(g Gomega) {
				cmd := "k3s etcd-snapshot save"
				if opts.compress {
					cmd += " --etcd-snapshot-compress=true"
				}
				if opts.s3 {
					cmd += " --etcd-s3=true --etcd-s3-insecure=true --etcd-s3-bucket=test-bucket --etcd-s3-folder=test-folder --etcd-s3-endpoint=172.17.0.1:9090 --etcd-s3-skip-ssl-verify=true --etcd-s3-access-key=test"
				}
				if opts.path != "" {
					cmd = "mkdir -p " + opts.path + "; " + cmd + " --etcd-snapshot-dir=" + opts.path
				}
				_, err := config.Servers[0].RunCmdOnNode(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				if opts.path != "" {
					cmd = "ls " + opts.path
				} else {
					cmd = "ls " + defaultSnapshotPath
				}
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
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running"
				res, err := tests.RunCommand(cmd)
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
			cmd := "k3s server --cluster-reset"
			if opts.s3 {
				cmd += " --etcd-s3=true --etcd-s3-insecure=true --etcd-s3-bucket=test-bucket --etcd-s3-folder=test-folder --etcd-s3-endpoint=172.17.0.1:9090 --etcd-s3-skip-ssl-verify=true --etcd-s3-access-key=test --cluster-reset-restore-path=" + snapshotname
			} else {
				if opts.path != "" {
					cmd += " --cluster-reset-restore-path=" + filepath.Join(opts.path, snapshotname)
				} else {
					cmd += " --cluster-reset-restore-path=" + filepath.Join(defaultSnapshotPath, snapshotname)
				}
			}
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
				cmd := "rm -rf /var/lib/rancher/k3s/server/db/etcd"
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
			Eventually(func() error {
				return tests.AllPodsUp(config.KubeconfigFile, "kube-system")
			}, "120s", "5s").Should(Succeed())
		})

		It("Verifies that workload1 exists and workload2 does not", func() {
			cmd := "kubectl get pods"
			res, err := tests.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring("test-clusterip"))
			Expect(res).ShouldNot(ContainSubstring("test-nodeport"))
		})
	})

	AfterEach(func() {
		failed = failed || CurrentSpecReport().Failed()
	})

	AfterAll(func() {
		if config != nil && failed {
			AddReportEntry("journald-logs", docker.TailJournalLogs(1000, append(config.Servers, config.Agents...)))
		}
		if *ci || (config != nil && !failed) {
			Expect(config.Cleanup()).To(Succeed())
		}
		_, err := tests.RunCommand("docker rm -fv s3mock")
		Expect(err).NotTo(HaveOccurred())
	})
},
	Entry("with default args", snapshotOptions{}),
	Entry("with default path, compressed", snapshotOptions{compress: true}),
	Entry("with default path (explicit)", snapshotOptions{path: defaultSnapshotPath}),
	Entry("with default path (explicit), compressed", snapshotOptions{path: defaultSnapshotPath, compress: true}),
	Entry("with custom path", snapshotOptions{path: "/tmp/snapshots/"}),
	Entry("with custom path, compressed", snapshotOptions{path: "/tmp/snapshots/", compress: true}),
	Entry("with S3", snapshotOptions{s3: true}),
	Entry("with S3, compressed", snapshotOptions{s3: true, compress: true}),
)

// Checks if nodes match the expected status
// We use kubectl directly, because getting a NotReady node status from the API is not easy
func CheckNodeStatus(kubeconfigFile string, readyNodes, notReadyNodes []string) error {
	readyNodesSet := set.New(readyNodes...)
	notReadyNodesSet := set.New(notReadyNodes...)
	foundReadyNodes := make(set.Set[string], 0)
	foundNotReadyNodes := make(set.Set[string], 0)

	cmd := "kubectl get nodes --no-headers --kubeconfig=" + kubeconfigFile
	res, err := tests.RunCommand(cmd)
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
