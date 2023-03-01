package longhorn

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var server *testutil.K3sServer
var serverArgs = []string{"--cluster-init"}
var testLock int

var _ = BeforeSuite(func() {
	if _, err := exec.LookPath("iscsiadm"); err != nil {
		Skip("Test needs open-iscsi to ben installed")
	} else if !testutil.IsExistingServer() {
		var err error
		testLock, err = testutil.K3sTestLock()
		Expect(err).ToNot(HaveOccurred())
		server, err = testutil.K3sStartServer(serverArgs...)
		Expect(err).ToNot(HaveOccurred())
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("longhorn", Ordered, func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(serverArgs) {
			Skip("Test needs k3s server with: " + strings.Join(serverArgs, " "))
		}
	})

	When("a new cluster is created", func() {
		It("starts up with no problems", func() {
			Eventually(func() error {
				return testutil.K3sDefaultDeployments()
			}, "120s", "5s").Should(Succeed())
		})
	})

	When("longhorn is installed", func() {
		It("installs components into the longhorn-system namespace", func() {
			result, err := testutil.K3sCmd("kubectl apply -f ./testdata/longhorn.yaml")
			Expect(result).To(ContainSubstring("namespace/longhorn-system created"))
			Expect(result).To(ContainSubstring("daemonset.apps/longhorn-manager created"))
			Expect(result).To(ContainSubstring("deployment.apps/longhorn-driver-deployer created"))
			Expect(result).To(ContainSubstring("deployment.apps/longhorn-recovery-backend created"))
			Expect(result).To(ContainSubstring("deployment.apps/longhorn-ui created"))
			Expect(result).To(ContainSubstring("deployment.apps/longhorn-conversion-webhook created"))
			Expect(result).To(ContainSubstring("deployment.apps/longhorn-admission-webhook created"))
			Expect(err).NotTo(HaveOccurred())
		})
		It("starts all pods with no problems", func() {
			Eventually(func() error {
				pods, err := testutil.ParsePods("longhorn-system", metav1.ListOptions{})
				if err != nil {
					return err
				}
				for _, pod := range pods {
					fmt.Println(pod)
					if pod.Status.Phase != "Running" && pod.Status.Phase != "Succeeded" {
						return fmt.Errorf("pod %s failing", pod.Name)
					}
				}
				return nil
			}, "120s", "5s").Should(Succeed())
		})
	})

	When("persistent volume claim is created", func() {
		It("creates the pv and pvc", func() {
			result, err := testutil.K3sCmd("kubectl create -f ./testdata/pvc.yaml")
			Expect(result).To(ContainSubstring("persistentvolumeclaim/longhorn-volv-pvc created"))
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() error {
				pvc, err := testutil.GetPersistentVolumeClaim("default", "longhorn-volv-pvc")
				if err != nil {
					return fmt.Errorf("failed to get pvc longhorn-volv-pvc")
				}
				if pvc.Status.Phase != "Bound" {
					return fmt.Errorf("pvc longhorn-volv-pvc not bound")
				}
				pv, err := testutil.GetPersistentVolume(pvc.Spec.VolumeName)
				if err != nil {
					return fmt.Errorf("failed to get pv %s", pvc.Spec.VolumeName)
				}
				if pv.Status.Phase != "Bound" {
					return fmt.Errorf("pv %s not bound", pv.Name)
				}
				return nil
			}, "60s", "5s").Should(Succeed())
		})
		It("creates a pod with the pvc", func() {
			result, err := testutil.K3sCmd("kubectl create -f ./testdata/pod.yaml")
			Expect(result).To(ContainSubstring("pod/volume-test created"))
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() error {
				pod, err := testutil.GetPod("default", "volume-test")
				if err != nil {
					return fmt.Errorf("failed to get pod volume-test")
				}
				if pod.Status.Phase != "Running" {
					return fmt.Errorf("pod volume-test not running")
				}
				return nil
			}, "30s", "5s").Should(Succeed())
		})
	})
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		if CurrentSpecReport().Failed() {
			testutil.K3sSaveLog(server, true)
		}
		Expect(testutil.K3sKillServer(server)).To(Succeed())
		Expect(testutil.K3sCleanup(testLock, "")).To(Succeed())
	}
})

func Test_IntegrationLonghorn(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Longhorn Suite")
}
