package integration

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	testutil "github.com/rancher/k3s/tests/util"
)

var server *testutil.K3sServer
var serverArgs = []string{"--cluster-init"}
var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		server, err = testutil.K3sStartServer(serverArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("local storage", func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(serverArgs) {
			Skip("Test needs k3s server with: " + strings.Join(serverArgs, " "))
		}
	})
	When("a new local storage is created", func() {
		It("starts up with no problems", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "pods", "-A")
			}, "90s", "1s").Should(MatchRegexp("kube-system.+coredns.+1\\/1.+Running"))
		})
		It("creates a new pvc", func() {
			result, err := testutil.K3sCmd("kubectl", "create", "-f", "../testdata/localstorage_pvc.yaml")
			Expect(result).To(ContainSubstring("persistentvolumeclaim/local-path-pvc created"))
			Expect(err).NotTo(HaveOccurred())
		})
		It("creates a new pod", func() {
			Expect(testutil.K3sCmd("kubectl", "create", "-f", "../testdata/localstorage_pod.yaml")).
				To(ContainSubstring("pod/volume-test created"))
		})
		It("shows storage up in kubectl", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "--namespace=default", "pvc")
			}, "45s", "1s").Should(MatchRegexp(`local-path-pvc.+Bound`))
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "--namespace=default", "pv")
			}, "10s", "1s").Should(MatchRegexp(`pvc.+2Gi.+Bound`))
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "--namespace=default", "pod")
			}, "10s", "1s").Should(MatchRegexp(`volume-test.+Running`))
		})
		It("has proper folder permissions", func() {
			var k3sStorage = "/var/lib/rancher/k3s/storage"
			fileStat, err := os.Stat(k3sStorage)
			Expect(err).ToNot(HaveOccurred())
			Expect(fmt.Sprintf("%04o", fileStat.Mode().Perm())).To(Equal("0701"))

			pvResult, err := testutil.K3sCmd("kubectl", "get", "--namespace=default", "pv")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`pvc[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			volumeName := reg.FindString(pvResult) + "_default_local-path-pvc"
			fileStat, err = os.Stat(k3sStorage + "/" + volumeName)
			Expect(err).ToNot(HaveOccurred())
			Expect(fmt.Sprintf("%04o", fileStat.Mode().Perm())).To(Equal("0777"))
		})
		It("deletes properly", func() {
			Expect(testutil.K3sCmd("kubectl", "delete", "--namespace=default", "--force", "pod", "volume-test")).
				To(ContainSubstring("pod \"volume-test\" force deleted"))
			Expect(testutil.K3sCmd("kubectl", "delete", "--namespace=default", "pvc", "local-path-pvc")).
				To(ContainSubstring("persistentvolumeclaim \"local-path-pvc\" deleted"))
		})
	})
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		Expect(testutil.K3sKillServer(server)).To(Succeed())
	}
})

func Test_IntegrationLocalStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Local Storage Suite", []Reporter{
		reporters.NewJUnitReporter("/tmp/results/junit-ls.xml"),
	})
}
