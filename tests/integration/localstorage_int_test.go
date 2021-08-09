package integration

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	testutil "github.com/rancher/k3s/tests/util"
)

var serverCmd *exec.Cmd
var _ = BeforeSuite(func() {
	var err error
	serverCmd, _, err = testutil.K3sCmdAsync("server", "--cluster-init")
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("local storage", func() {
	When("a new local storage is created", func() {
		It("starts up with no problems", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "pods", "-A")
			}, "90s", "1s").Should(MatchRegexp("kube-system.+coredns.+1\\/1.+Running"))
		})
		It("creates a new pvc", func() {
			Expect(testutil.K3sCmd("kubectl", "create", "-f", "../testdata/localstorage_pvc.yaml")).
				To(ContainSubstring("persistentvolumeclaim/local-path-pvc created"))
		})
		It("creates a new pod", func() {
			Expect(testutil.K3sCmd("kubectl", "create", "-f", "../testdata/localstorage_pod.yaml")).
				To(ContainSubstring("pod/volume-test created"))
		})
		time.Sleep(30 * time.Second)
		It("shows storage up in kubectl", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "pv")
			}, "30s", "1s").Should(MatchRegexp(`pvc.+2Gi.+Bound`))
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "pvc")
			}, "10s", "1s").Should(MatchRegexp(`local-path-pvc.+Bound`))
		})
		It("has proper folder permissions", func() {
			var k3sStorage = "/var/lib/rancher/k3s/storage"
			fileStat, err := os.Stat(k3sStorage)
			Expect(err).ToNot(HaveOccurred())
			Expect(fmt.Sprintf("%04o", fileStat.Mode().Perm())).To(Equal("0701"))

			pvResult, err := testutil.K3sCmd("kubectl", "get", "pv")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`pvc[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			volumeName := reg.FindString(pvResult) + "_default_local-path-pvc"
			fileStat, err = os.Stat(k3sStorage + "/" + volumeName)
			Expect(err).ToNot(HaveOccurred())
			Expect(fmt.Sprintf("%04o", fileStat.Mode().Perm())).To(Equal("0777"))
		})
		It("deletes properly", func() {
			Expect(testutil.K3sCmd("kubectl", "delete", "pod", "volume-test")).
				To(ContainSubstring("pod \"volume-test\" deleted"))
			Expect(testutil.K3sCmd("kubectl", "delete", "pvc", "local-path-pvc")).
				To(ContainSubstring("persistentvolumeclaim \"local-path-pvc\" deleted"))
		})
	})
})

var _ = AfterSuite(func() {
	Expect(testutil.K3sKillAsync(serverCmd)).To(Succeed())
})

func Test_IntegrationLocalStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Local Storage Suite")

}
