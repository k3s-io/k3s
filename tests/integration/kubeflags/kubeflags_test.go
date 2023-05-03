package kubeflags

import (
	"strings"
	"testing"

	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var server *testutil.K3sServer
var serverArgs = []string{"--cluster-init",
	"--kube-apiserver-arg", "advertise-port=1234",
	"--kube-controller-manager-arg", "allocate-node-cidrs=false",
	"--kube-scheduler-arg", "authentication-kubeconfig=test",
	"--kube-cloud-controller-manager-arg", "allocate-node-cidrs=false",
	"--kubelet-arg", "address=127.0.0.1",
	"--kube-proxy-arg", "cluster-cidr=127.0.0.1/16",
}
var testLock int

var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		testLock, err = testutil.K3sTestLock()
		Expect(err).ToNot(HaveOccurred())
		server, err = testutil.K3sStartServer(serverArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("create a new cluster with kube-* flags", Ordered, func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(serverArgs) {
			Skip("Test needs k3s server with: " + strings.Join(serverArgs, " "))
		}
	})
	When("should print the args on the console", func() {
		It("should find cloud-controller-manager starting", func() {
			Eventually(func() error {
				match, err := testutil.SearchK3sLog(server, "Running cloud-controller-manager --allocate-node-cidrs=false")
				if err != nil {
					return err
				}
				if match {
					return nil
				}
				return errors.New("error finding cloud-controller-manager")
			}, "30s", "2s").Should(Succeed())
		})
		It("should find kube-scheduler starting", func() {
			Eventually(func() error {
				match, err := testutil.SearchK3sLog(server, "Running kube-scheduler --authentication-kubeconfig=test")
				if err != nil {
					return err
				}
				if match {
					return nil
				}
				return errors.New("error finding kube-scheduler")
			}, "30s", "2s").Should(Succeed())
		})
		It("should find kube-apiserver starting", func() {
			Eventually(func() error {
				match, err := testutil.SearchK3sLog(server, "Running kube-apiserver --advertise-port=1234")
				if err != nil {
					return err
				}
				if match {
					return nil
				}
				return errors.New("error finding kube-apiserver")
			}, "30s", "2s").Should(Succeed())
		})
		It("should find kube-controller-manager starting", func() {
			Eventually(func() error {
				match, err := testutil.SearchK3sLog(server, "Running kube-controller-manager --allocate-node-cidrs=false")
				if err != nil {
					return err
				}
				if match {
					return nil
				}
				return errors.New("error finding kube-controller-manager")
			}, "30s", "2s").Should(Succeed())
		})
		It("should find kubelet starting", func() {
			Eventually(func() error {
				match, err := testutil.SearchK3sLog(server, "Running kubelet --address=127.0.0.1")
				if err != nil {
					return err
				}
				if match {
					return nil
				}
				return errors.New("error finding kubelet")
			}, "120s", "15s").Should(Succeed())
		})
		It("should find kube-proxy starting", func() {
			Eventually(func() error {
				match, err := testutil.SearchK3sLog(server, "Running kube-proxy --cluster-cidr=127.0.0.1/16")
				if err != nil {
					return err
				}
				if match {
					return nil
				}
				return errors.New("error finding kube-proxy")
			}, "120s", "15s").Should(Succeed())
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
			testutil.K3sSaveLog(server, false)
		}
		Expect(testutil.K3sKillServer(server)).To(Succeed())
		Expect(testutil.K3sCleanup(testLock, "")).To(Succeed())
	}
})

func Test_IntegrationEtcdSnapshot(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Etcd Snapshot Suite")
}
