package scale

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var k3sImage = flag.String("k3sImage", "", "The image used to provision containers")
var ci = flag.Bool("ci", false, "running on CI, forced cleanup")

var serverCount = flag.Int("serverCount", 5, "number of server nodes")
var agentCount = flag.Int("agentCount", 50, "number of agent nodes")
var namespaceCount = flag.Int("namespaceCount", 10, "number of namespaces used for scale workload")
var deploymentsPerNamespace = flag.Int("deploymentsPerNamespace", 10, "deployments created per namespace")
var replicasPerDeployment = flag.Int("replicasPerDeployment", 15, "target replicas per deployment")

var config *docker.TestConfig
var scaleNamespaces []string

const maxExpectedPods = 5000

func Test_DockerScale(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scale Docker Test Suite")
}

var _ = Describe("Scale Tests", Ordered, func() {
	Context("Setup Cluster", func() {
		It("should provision a 5 server, 50 agent cluster", func() {
			var err error
			config, err = docker.NewTestConfig(*k3sImage)
			Expect(err).NotTo(HaveOccurred())

			Expect(config.ProvisionServers(*serverCount)).To(Succeed())
			Expect(config.ProvisionAgents(*agentCount)).To(Succeed())

			Eventually(func() error {
				return tests.CheckDefaultDeployments(config.KubeconfigFile)
			}, "10m", "10s").Should(Succeed())

			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "10m", "10s").Should(Succeed())

			res, err := tests.RunCommand("mpstat")
			Expect(err).NotTo(HaveOccurred())
			By("CPU Usage on Node Before Workload:\n%s" + res)
			res, err = tests.RunCommand("free -mh")
			Expect(err).NotTo(HaveOccurred())
			By("Memory Usage on Node Before Workload:\n%s" + res)
		})
	})

	Context("Large Deployment Workload", func() {
		expectedPods := *namespaceCount * *deploymentsPerNamespace * *replicasPerDeployment
		It("should create waves of deployments", func() {
			Expect(expectedPods).To(BeNumerically(">", 0))
			Expect(expectedPods).To(BeNumerically("<=", maxExpectedPods), "requested workload exceeds CNCF runners capacity")

			scaleNamespaces = make([]string, 0, *namespaceCount)
			for i := 0; i < *namespaceCount; i++ {
				ns := fmt.Sprintf("scale-%02d", i)
				scaleNamespaces = append(scaleNamespaces, ns)
				_, err := tests.RunCommand("kubectl create namespace " + ns)
				Expect(err).NotTo(HaveOccurred())

				for j := 0; j < *deploymentsPerNamespace; j++ {
					name := fmt.Sprintf("load-%02d", j)
					Expect(createDeployment(ns, name)).To(Succeed())
				}
			}
		})
		It("should scale up each deployment to the target replica count", func() {
			for i, ns := range scaleNamespaces {
				cmd := fmt.Sprintf("kubectl -n %s scale deployment --all --replicas=%d", ns, *replicasPerDeployment)
				_, err := tests.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() error {
					waitCmd := fmt.Sprintf("kubectl -n %s wait --for=condition=Available --timeout=120s deployment --all", ns)
					_, waitErr := tests.RunCommand(waitCmd)
					return waitErr
				}, "10m", "10s").Should(Succeed())

				time.Sleep(5 * time.Second) // brief pause between namespace waves to avoid overwhelming the cluster
				By("Scale progress: " + strconv.Itoa(i+1) + "/" + strconv.Itoa(*namespaceCount) + " namespaces scaled")
			}
		})
		It("should still have all pods running and nodes ready", func() {
			cmd := "kubectl get pods -A -l scale.k3s.io/test=docker-large --field-selector=status.phase=Running --no-headers | wc -l"
			Eventually(func() (int, error) {
				out, err := tests.RunCommand(cmd)
				if err != nil {
					return 0, err
				}
				return strconv.Atoi(strings.TrimSpace(out))
			}, "10m", "20s").Should(BeNumerically(">=", expectedPods))

			Eventually(func() error {
				return tests.NodesReady(config.KubeconfigFile, config.GetNodeNames())
			}, "1m", "10s").Should(Succeed())

			Eventually(func() error {
				return tests.AllPodsUp(config.KubeconfigFile, "kube-system")
			}, "1m", "10s").Should(Succeed())
		})
		It("should log the amount of Resources used by the cluster", func() {
			res, err := tests.RunCommand("mpstat")
			Expect(err).NotTo(HaveOccurred())
			By("CPU Usage on Node:\n%s" + res)
			res, err = tests.RunCommand("free -mh")
			Expect(err).NotTo(HaveOccurred())
			By("Memory Usage on Node:\n%s" + res)
		})
		It("should clean up all scale workloads", func() {
			for _, ns := range scaleNamespaces {
				_, err := tests.RunCommand("kubectl delete namespace " + ns)
				Expect(err).NotTo(HaveOccurred())
			}
			// Under 5 namespaces we can move on
			Eventually(func() (string, error) {
				cmd := "kubectl get namespaces -l scale.k3s.io/test=docker-large --no-headers"
				return tests.RunCommand(cmd)
			}, "10m", "20s").Should(ContainSubstring("No resources found"))
		})
	})
})

var failed bool

var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		AddReportEntry("describe", docker.DescribeNodesAndPods(config))
		AddReportEntry("docker-containers", docker.ListContainers())
		if config != nil {
			AddReportEntry("docker-logs", docker.TailDockerLogs(300, append(config.Servers, config.Agents...)))
		}
	}
	if config != nil && (*ci || !failed) {
		Expect(config.Cleanup()).To(Succeed())
	}
})

func createDeployment(namespace, name string) error {
	manifest := fmt.Sprintf(`cat <<'EOF' | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    scale.k3s.io/test: docker-large
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s
      scale.k3s.io/test: docker-large
  template:
    metadata:
      labels:
        app: %s
        scale.k3s.io/test: docker-large
    spec:
      containers:
      - name: web
        image: rancher/mirrored-library-nginx:1.29.1-alpine
        resources:
          requests:
            cpu: 100m
            memory: 16M
EOF`, name, namespace, name, name)

	_, err := tests.RunCommand(manifest)
	return err
}
