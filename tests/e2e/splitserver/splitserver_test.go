package splitserver

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/e2e"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sync/errgroup"
)

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var etcdCount = flag.Int("etcdCount", 3, "number of server nodes only deploying etcd")
var controlPlaneCount = flag.Int("controlPlaneCount", 1, "number of server nodes acting as control plane")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built K3s binary")
var hardened = flag.Bool("hardened", false, "true or false")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+k3s2 or nil for latest commit from master

// createSplitCluster creates a split server cluster with the given nodeOS, etcdCount, controlPlaneCount, and agentCount.
// It duplicates and merges functionality found in the e2e.CreateCluster and e2e.CreateLocalCluster functions.
func createSplitCluster(nodeOS string, etcdCount, controlPlaneCount, agentCount int, local bool) ([]e2e.VagrantNode, []e2e.VagrantNode, []e2e.VagrantNode, error) {
	etcdNodes := make([]e2e.VagrantNode, etcdCount)
	cpNodes := make([]e2e.VagrantNode, controlPlaneCount)
	agentNodes := make([]e2e.VagrantNode, agentCount)

	for i := 0; i < etcdCount; i++ {
		etcdNodes[i] = e2e.VagrantNode("server-etcd-" + strconv.Itoa(i))
	}
	for i := 0; i < controlPlaneCount; i++ {
		cpNodes[i] = e2e.VagrantNode("server-cp-" + strconv.Itoa(i))
	}
	for i := 0; i < agentCount; i++ {
		agentNodes[i] = e2e.VagrantNode("agent-" + strconv.Itoa(i))
	}
	nodeRoles := strings.Join(e2e.VagrantSlice(etcdNodes), " ") + " " + strings.Join(e2e.VagrantSlice(cpNodes), " ") + " " + strings.Join(e2e.VagrantSlice(agentNodes), " ")

	nodeRoles = strings.TrimSpace(nodeRoles)
	nodeBoxes := strings.Repeat(nodeOS+" ", etcdCount+controlPlaneCount+agentCount)
	nodeBoxes = strings.TrimSpace(nodeBoxes)

	allNodeNames := append(e2e.VagrantSlice(etcdNodes), e2e.VagrantSlice(cpNodes)...)
	allNodeNames = append(allNodeNames, e2e.VagrantSlice(agentNodes)...)

	var testOptions string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}

	// Provision the first etcd node. In GitHub Actions, this also imports the VM image into libvirt, which
	// takes time and can cause the next vagrant up to fail if it is not given enough time to complete.
	cmd := fmt.Sprintf(`E2E_NODE_ROLES="%s" E2E_NODE_BOXES="%s" vagrant up --no-provision %s &> vagrant.log`, nodeRoles, nodeBoxes, etcdNodes[0].String())
	fmt.Println(cmd)
	if _, err := e2e.RunCommand(cmd); err != nil {
		return etcdNodes, cpNodes, agentNodes, err
	}

	// Bring up the rest of the nodes in parallel
	errg, _ := errgroup.WithContext(context.Background())
	for _, node := range allNodeNames[1:] {
		cmd := fmt.Sprintf(`E2E_NODE_ROLES="%s" E2E_NODE_BOXES="%s" vagrant up --no-provision %s &>> vagrant.log`, nodeRoles, nodeBoxes, node)
		errg.Go(func() error {
			_, err := e2e.RunCommand(cmd)
			return err
		})
		// libVirt/Virtualbox needs some time between provisioning nodes
		time.Sleep(10 * time.Second)
	}
	if err := errg.Wait(); err != nil {
		return etcdNodes, cpNodes, agentNodes, err
	}

	if local {
		testOptions += " E2E_RELEASE_VERSION=skip"
		for _, node := range allNodeNames {
			cmd := fmt.Sprintf(`E2E_NODE_ROLES=%s vagrant scp ../../../dist/artifacts/k3s  %s:/tmp/`, node, node)
			if _, err := e2e.RunCommand(cmd); err != nil {
				return etcdNodes, cpNodes, agentNodes, fmt.Errorf("failed to scp k3s binary to %s: %v", node, err)
			}
			cmd = fmt.Sprintf(`E2E_NODE_ROLES=%s vagrant ssh %s -c "sudo mv /tmp/k3s /usr/local/bin/"`, node, node)
			if _, err := e2e.RunCommand(cmd); err != nil {
				return etcdNodes, cpNodes, agentNodes, err
			}
		}
	}
	// Install K3s on all nodes in parallel
	errg, _ = errgroup.WithContext(context.Background())
	for _, node := range allNodeNames {
		cmd = fmt.Sprintf(`E2E_NODE_ROLES="%s" E2E_NODE_BOXES="%s" %s vagrant provision %s &>> vagrant.log`, nodeRoles, nodeBoxes, testOptions, node)
		errg.Go(func() error {
			_, err := e2e.RunCommand(cmd)
			return err
		})
		// K3s needs some time between joining nodes to avoid learner issues
		time.Sleep(10 * time.Second)
	}
	if err := errg.Wait(); err != nil {
		return etcdNodes, cpNodes, agentNodes, err
	}
	return etcdNodes, cpNodes, agentNodes, nil
}

func Test_E2ESplitServer(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Split Server Test Suite", suiteConfig, reporterConfig)
}

var (
	tc         *e2e.TestConfig // We don't use the Server and Agents from this
	etcdNodes  []e2e.VagrantNode
	cpNodes    []e2e.VagrantNode
	agentNodes []e2e.VagrantNode
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Create", Ordered, func() {
	Context("Cluster :", func() {
		It("Starts up with no issues", func() {
			var err error
			etcdNodes, cpNodes, agentNodes, err = createSplitCluster(*nodeOS, *etcdCount, *controlPlaneCount, *agentCount, *local)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			fmt.Println("CLUSTER CONFIG")
			fmt.Println("OS:", *nodeOS)
			fmt.Println("Etcd Server Nodes:", etcdNodes)
			fmt.Println("Control Plane Server Nodes:", cpNodes)
			fmt.Println("Agent Nodes:", agentNodes)
			kubeConfigFile, err := e2e.GenKubeconfigFile(cpNodes[0].String())
			tc = &e2e.TestConfig{
				KubeconfigFile: kubeConfigFile,
				Hardened:       *hardened,
			}
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status", func() {
			By("Fetching Nodes status")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())

			Eventually(func() error {
				return tests.AllPodsUp(tc.KubeconfigFile)
			}, "620s", "5s").Should(Succeed())
			e2e.DumpPods(tc.KubeconfigFile)
		})

		It("Verifies ClusterIP Service", func() {
			_, err := tc.DeployWorkload("clusterip.yaml")
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")

			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "240s", "5s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)

			clusterip, _ := e2e.FetchClusterIP(tc.KubeconfigFile, "nginx-clusterip-svc", false)
			cmd = "curl -m 5 -s -f http://" + clusterip + "/name.html"
			for _, node := range cpNodes {
				Eventually(func() (string, error) {
					return node.RunCmdOnNode(cmd)
				}, "120s", "10s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)
			}
		})

		It("Verifies NodePort Service", func() {
			_, err := tc.DeployWorkload("nodeport.yaml")
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")

			for _, node := range cpNodes {
				nodeExternalIP, _ := node.FetchNodeExternalIP()
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")

				cmd = "curl -m 5 -s -f http://" + nodeExternalIP + ":" + nodeport + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "failed cmd: "+cmd)
			}
		})

		It("Verifies LoadBalancer Service", func() {
			_, err := tc.DeployWorkload("loadbalancer.yaml")
			Expect(err).NotTo(HaveOccurred(), "Loadbalancer manifest not deployed")

			for _, node := range cpNodes {
				ip, _ := node.FetchNodeExternalIP()

				cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].port}\""
				port, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"), "failed cmd: "+cmd)

				cmd = "curl -m 5 -s -f http://" + ip + ":" + port + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"), "failed cmd: "+cmd)
			}
		})

		It("Verifies Ingress", func() {
			_, err := tc.DeployWorkload("ingress.yaml")
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")

			for _, node := range cpNodes {
				ip, _ := node.FetchNodeExternalIP()
				cmd := "curl --header host:foo1.bar.com -m 5 -s -f http://" + ip + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-ingress"), "failed cmd: "+cmd)
			}
		})

		It("Verifies Daemonset", func() {
			_, err := tc.DeployWorkload("daemonset.yaml")
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")

			Eventually(func(g Gomega) {
				count, err := e2e.GetDaemonsetReady("test-daemonset", tc.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cpNodes).To(HaveLen(count), "Daemonset pod count does not match cp node count")
			}, "240s", "10s").Should(Succeed())
		})

		It("Verifies dns access", func() {
			_, err := tc.DeployWorkload("dnsutils.yaml")
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")

			cmd := "kubectl get pods dnsutils --kubeconfig=" + tc.KubeconfigFile
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("dnsutils"), "failed cmd: "+cmd)

			cmd = "kubectl --kubeconfig=" + tc.KubeconfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("kubernetes.default.svc.cluster.local"), "failed cmd: "+cmd)
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	allNodes := append(cpNodes, etcdNodes...)
	allNodes = append(allNodes, agentNodes...)
	if failed {
		AddReportEntry("journald-logs", e2e.TailJournalLogs(1000, allNodes))
	} else {
		Expect(e2e.GetCoverageReport(allNodes)).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
