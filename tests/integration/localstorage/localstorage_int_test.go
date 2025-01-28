package integration

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	tests "github.com/k3s-io/k3s/tests"
	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var localStorageServer *testutil.K3sServer
var localStorageServerArgs = []string{"--cluster-init"}
var testLock int

var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		testLock, err = testutil.K3sTestLock()
		Expect(err).ToNot(HaveOccurred())
		localStorageServer, err = testutil.K3sStartServer(localStorageServerArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("local storage", Ordered, func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(localStorageServerArgs) {
			Skip("Test needs k3s server with: " + strings.Join(localStorageServerArgs, " "))
		}
	})
	When("a new local storage is created", func() {
		It("starts up with no problems", func() {
			Eventually(func() error {
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "120s", "5s").Should(Succeed())
		})
		It("creates a new pvc", func() {
			Expect(testutil.K3sCmd("kubectl create -f ./testdata/localstorage_pvc.yaml")).
				To(ContainSubstring("persistentvolumeclaim/local-path-pvc created"))
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
			Expect(fmt.Sprintf("%04o", fileStat.Mode().Perm())).To(Equal("0700"))

			pvResult, err := testutil.K3sCmd("kubectl get --namespace=default pv")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`pvc[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			volumeName := reg.FindString(pvResult) + "_default_local-path-pvc"
			fileStat, err = os.Stat(k3sStorage + "/" + volumeName)
			Expect(err).ToNot(HaveOccurred())
			Expect(fmt.Sprintf("%04o", fileStat.Mode().Perm())).To(Equal("0777"))

			Eventually(func() error {
				_, err = os.Stat(k3sStorage + "/" + volumeName + "/file1")
				return err
			}, "10s", "1s").Should(Succeed())
			Expect(testutil.K3sCmd("kubectl --namespace=default exec volume-test -- stat -c %a /data/file1")).
				To(Equal("644\n"))

		})
		It("allows non-root pods to write to the volume", func() {
			Expect(testutil.K3sCmd("kubectl --namespace=default exec volume-test -- touch /data/file2")).
				To(BeEmpty())
			Expect(testutil.K3sCmd("kubectl --namespace=default exec volume-test -- stat -c %a /data/file2")).
				To(Equal("644\n"))
		})
		It("deletes properly", func() {
			Expect(testutil.K3sCmd("kubectl delete --namespace=default --force pod volume-test")).
				To(ContainSubstring("pod \"volume-test\" force deleted"))
			Expect(testutil.K3sCmd("kubectl delete --namespace=default pvc local-path-pvc")).
				To(ContainSubstring("persistentvolumeclaim \"local-path-pvc\" deleted"))
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		if failed {
			testutil.K3sSaveLog(localStorageServer, false)
		}
		Expect(testutil.K3sKillServer(localStorageServer)).To(Succeed())
		Expect(testutil.K3sCleanup(testLock, "")).To(Succeed())
	}
})

func Test_IntegrationLocalStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Local Storage Suite")
}
