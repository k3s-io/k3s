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
func createSplitCluster(nodeOS string, etcdCount, controlPlaneCount, agentCount int, local bool) ([]string, []string, []string, error) {
	etcdNodeNames := make([]string, etcdCount)
	cpNodeNames := make([]string, controlPlaneCount)
	agentNodeNames := make([]string, agentCount)

	for i := 0; i < etcdCount; i++ {
		etcdNodeNames[i] = "server-etcd-" + strconv.Itoa(i)
	}
	for i := 0; i < controlPlaneCount; i++ {
		cpNodeNames[i] = "server-cp-" + strconv.Itoa(i)
	}
	for i := 0; i < agentCount; i++ {
		agentNodeNames[i] = "agent-" + strconv.Itoa(i)
	}
	nodeRoles := strings.Join(etcdNodeNames, " ") + " " + strings.Join(cpNodeNames, " ") + " " + strings.Join(agentNodeNames, " ")

	nodeRoles = strings.TrimSpace(nodeRoles)
	nodeBoxes := strings.Repeat(nodeOS+" ", etcdCount+controlPlaneCount+agentCount)
	nodeBoxes = strings.TrimSpace(nodeBoxes)

	allNodes := append(etcdNodeNames, cpNodeNames...)
	allNodes = append(allNodes, agentNodeNames...)

	var testOptions string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}

	// Provision the first etcd node. In GitHub Actions, this also imports the VM image into libvirt, which
	// takes time and can cause the next vagrant up to fail if it is not given enough time to complete.
	cmd := fmt.Sprintf(`E2E_NODE_ROLES="%s" E2E_NODE_BOXES="%s" vagrant up --no-provision %s &> vagrant.log`, nodeRoles, nodeBoxes, etcdNodeNames[0])
	fmt.Println(cmd)
	if _, err := e2e.RunCommand(cmd); err != nil {
		return etcdNodeNames, cpNodeNames, agentNodeNames, err
	}

	// Bring up the rest of the nodes in parallel
	errg, _ := errgroup.WithContext(context.Background())
	for _, node := range allNodes[1:] {
		cmd := fmt.Sprintf(`E2E_NODE_ROLES="%s" E2E_NODE_BOXES="%s" vagrant up --no-provision %s &>> vagrant.log`, nodeRoles, nodeBoxes, node)
		errg.Go(func() error {
			_, err := e2e.RunCommand(cmd)
			return err
		})
		// libVirt/Virtualbox needs some time between provisioning nodes
		time.Sleep(10 * time.Second)
	}
	if err := errg.Wait(); err != nil {
		return etcdNodeNames, cpNodeNames, agentNodeNames, err
	}

	if local {
		testOptions += " E2E_RELEASE_VERSION=skip"
		for _, node := range allNodes {
			cmd := fmt.Sprintf(`E2E_NODE_ROLES=%s vagrant scp ../../../dist/artifacts/k3s  %s:/tmp/`, node, node)
			if _, err := e2e.RunCommand(cmd); err != nil {
				return etcdNodeNames, cpNodeNames, agentNodeNames, fmt.Errorf("failed to scp k3s binary to %s: %v", node, err)
			}
			if _, err := e2e.RunCmdOnNode("mv /tmp/k3s /usr/local/bin/", node); err != nil {
				return etcdNodeNames, cpNodeNames, agentNodeNames, err
			}
		}
	}
	// Install K3s on all nodes in parallel
	errg, _ = errgroup.WithContext(context.Background())
	for _, node := range allNodes {
		cmd = fmt.Sprintf(`E2E_NODE_ROLES="%s" E2E_NODE_BOXES="%s" %s vagrant provision %s &>> vagrant.log`, nodeRoles, nodeBoxes, testOptions, node)
		errg.Go(func() error {
			_, err := e2e.RunCommand(cmd)
			return err
		})
		// K3s needs some time between joining nodes to avoid learner issues
		time.Sleep(10 * time.Second)
	}
	if err := errg.Wait(); err != nil {
		return etcdNodeNames, cpNodeNames, agentNodeNames, err
	}
	return etcdNodeNames, cpNodeNames, agentNodeNames, nil
}

func Test_E2ESplitServer(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Split Server Test Suite", suiteConfig, reporterConfig)
}

var (
	kubeConfigFile string
	etcdNodeNames  []string
	cpNodeNames    []string
	agentNodeNames []string
)

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Create", Ordered, func() {
	Context("Cluster :", func() {
		It("Starts up with no issues", func() {
			var err error
			etcdNodeNames, cpNodeNames, agentNodeNames, err = createSplitCluster(*nodeOS, *etcdCount, *controlPlaneCount, *agentCount, *local)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			fmt.Println("CLUSTER CONFIG")
			fmt.Println("OS:", *nodeOS)
			fmt.Println("Etcd Server Nodes:", etcdNodeNames)
			fmt.Println("Control Plane Server Nodes:", cpNodeNames)
			fmt.Println("Agent Nodes:", agentNodeNames)
			kubeConfigFile, err = e2e.GenKubeConfigFile(cpNodeNames[0])
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

		It("Verifies ClusterIP Service", func() {
			_, err := e2e.DeployWorkload("clusterip.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")

			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "240s", "5s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)

			clusterip, _ := e2e.FetchClusterIP(kubeConfigFile, "nginx-clusterip-svc", false)
			cmd = "curl -L --insecure http://" + clusterip + "/name.html"
			for _, nodeName := range cpNodeNames {
				Eventually(func() (string, error) {
					return e2e.RunCmdOnNode(cmd, nodeName)
				}, "120s", "10s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)
			}
		})

		It("Verifies NodePort Service", func() {
			_, err := e2e.DeployWorkload("nodeport.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")

			for _, nodeName := range cpNodeNames {
				nodeExternalIP, _ := e2e.FetchNodeExternalIP(nodeName)
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")

				cmd = "curl -L --insecure http://" + nodeExternalIP + ":" + nodeport + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "failed cmd: "+cmd)
			}
		})

		It("Verifies LoadBalancer Service", func() {
			_, err := e2e.DeployWorkload("loadbalancer.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "Loadbalancer manifest not deployed")

			for _, nodeName := range cpNodeNames {
				ip, _ := e2e.FetchNodeExternalIP(nodeName)

				cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].port}\""
				port, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"), "failed cmd: "+cmd)

				cmd = "curl -L --insecure http://" + ip + ":" + port + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"), "failed cmd: "+cmd)
			}
		})

		It("Verifies Ingress", func() {
			_, err := e2e.DeployWorkload("ingress.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")

			for _, nodeName := range cpNodeNames {
				ip, _ := e2e.FetchNodeExternalIP(nodeName)
				cmd := "curl  --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-ingress"), "failed cmd: "+cmd)
			}
		})

		It("Verifies Daemonset", func() {
			_, err := e2e.DeployWorkload("daemonset.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")

			Eventually(func(g Gomega) {
				pods, _ := e2e.ParsePods(kubeConfigFile, false)
				count := e2e.CountOfStringInSlice("test-daemonset", pods)
				fmt.Println("POD COUNT")
				fmt.Println(count)
				fmt.Println("CP COUNT")
				fmt.Println(len(cpNodeNames))
				g.Expect(len(cpNodeNames)).Should((Equal(count)), "Daemonset pod count does not match cp node count")
			}, "240s", "10s").Should(Succeed())
		})

		It("Verifies dns access", func() {
			_, err := e2e.DeployWorkload("dnsutils.yaml", kubeConfigFile, *hardened)
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")

			cmd := "kubectl get pods dnsutils --kubeconfig=" + kubeConfigFile
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("dnsutils"), "failed cmd: "+cmd)

			cmd = "kubectl --kubeconfig=" + kubeConfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"
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
	if !failed {
		allNodes := append(cpNodeNames, etcdNodeNames...)
		allNodes = append(allNodes, agentNodeNames...)
		Expect(e2e.GetCoverageReport(allNodes)).To(Succeed())
	}
	if !failed || *ci {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
