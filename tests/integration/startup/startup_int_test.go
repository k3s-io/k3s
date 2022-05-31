package integration

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var startupServer *testutil.K3sServer
var startupServerArgs = []string{}
var testLock int

var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		testLock, err = testutil.K3sTestLock()
		Expect(err).ToNot(HaveOccurred())

	}
})

var _ = Describe("startup tests", func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() {
			Skip("Test does not support running on existing k3s servers")
		}
	})
	When("a default server is created", func() {
		It("is created with no arguments", func() {
			var err error
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods deployed", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "60s", "5s").Should(Succeed())
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
		})
	})
	When("a etcd backed server is created", func() {
		It("is created with cluster-init arguments", func() {
			var err error
			startupServerArgs = []string{"--cluster-init"}
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods deployed", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "60s", "5s").Should(Succeed())
		})
		It("dies cleanly", func() {
			Expect(testutil.K3sKillServer(startupServer)).To(Succeed())
		})
	})
	When("a server without traefik is created", func() {
		It("is created with disable arguments", func() {
			var err error
			startupServerArgs = []string{"--disable", "traefik"}
			startupServer, err = testutil.K3sStartServer(startupServerArgs...)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has the default pods without traefik deployed", func() {
			Eventually(func() error {
				return testutil.K3sCheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server"})
			}, "60s", "5s").Should(Succeed())
		})
		It("creates a new pvc", func() {
			result, err := testutil.K3sCmd("kubectl create -f ./testdata/localstorage_pvc.yaml")
			Expect(result).To(ContainSubstring("persistentvolumeclaim/local-path-pvc created"))
			Expect(err).NotTo(HaveOccurred())
		})
		It("creates a new pod", func() {
			Expect(testutil.K3sCmd("kubectl create -f ./testdata/localstorage_pod.yaml")).
				To(ContainSubstring("pod/volume-test created"))
		})
		It("shows storage up in kubectl", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl get --namespace=default pvc")
			}, "45s", "1s").Should(MatchRegexp(`local-path-pvc.+Bound`))
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl get --namespace=default pv")
			}, "10s", "1s").Should(MatchRegexp(`pvc.+1Gi.+Bound`))
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl get --namespace=default pod")
			}, "10s", "1s").Should(MatchRegexp(`volume-test.+Running`))
		})
		It("has proper folder permissions", func() {
			var k3sStorage = "/var/lib/rancher/k3s/storage"
			fileStat, err := os.Stat(k3sStorage)
			Expect(err).ToNot(HaveOccurred())
			Expect(fmt.Sprintf("%04o", fileStat.Mode().Perm())).To(Equal("0701"))

			pvResult, err := testutil.K3sCmd("kubectl get --namespace=default pv")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`pvc[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			volumeName := reg.FindString(pvResult) + "_default_local-path-pvc"
			fileStat, err = os.Stat(k3sStorage + "/" + volumeName)
			Expect(err).ToNot(HaveOccurred())
			Expect(fmt.Sprintf("%04o", fileStat.Mode().Perm())).To(Equal("0777"))
		})
		It("deletes properly", func() {
			Expect(testutil.K3sCmd("kubectl delete --namespace=default --force pod volume-test")).
				To(ContainSubstring("pod \"volume-test\" force deleted"))
			Expect(testutil.K3sCmd("kubectl delete --namespace=default pvc local-path-pvc")).
				To(ContainSubstring("persistentvolumeclaim \"local-path-pvc\" deleted"))
		})
	})
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		Expect(testutil.K3sCleanup(testLock, "")).To(Succeed())
	}
})

func Test_IntegrationStartup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Startup Suite")
}
