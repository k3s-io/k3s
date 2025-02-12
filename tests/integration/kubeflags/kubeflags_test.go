package kubeflags

import (
	"strings"
	"testing"

	tests "github.com/k3s-io/k3s/tests"
	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var server *testutil.K3sServer
var serverArgs = []string{"--cluster-init",
	"--kube-apiserver-arg", "advertise-port=1234",
	"--kube-controller-manager-arg", "allocate-node-cidrs=false",
	"--kube-scheduler-arg", "allow-metric-labels=metric1,label1='v3'",
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
		if testutil.IsExistingServer() {
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
				match, err := testutil.SearchK3sLog(server, "Running kube-scheduler --allow-metric-labels=metric1,label1='v3'")
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
			}, "300s", "15s").Should(Succeed())
		})
		When("server is setup without kube-proxy or cloud-controller-manager ", func() {
			It("kills previous server and clean logs", func() {
				Expect(testutil.K3sKillServer(server)).To(Succeed())
			})
			It("start up with disabled kube-proxy and cloud controller", func() {
				var err error
				localServerArgs := []string{
					"--cluster-init",
					"--disable-cloud-controller=true",
					"--disable-kube-proxy=true",
				}
				server, err = testutil.K3sStartServer(localServerArgs...)
				Expect(err).ToNot(HaveOccurred())

				// Pods should not be healthy without kube-proxy
				Consistently(func() error {
					return tests.CheckDefaultDeployments(testutil.DefaultConfig)
				}, "100s", "5s").Should(HaveOccurred())
			})
			It("should not find kube-proxy starting", func() {
				Consistently(func() error {
					match, err := testutil.SearchK3sLog(server, "Running kube-proxy")
					if err != nil {
						return err
					}
					if !match {
						return nil
					}
					return errors.New("found kube-proxy starting")
				}, "100s", "5s").Should(Succeed())
			})
			/* The flag --disable-cloud-controller doesn't stop ccm from running,
			it appends -cloud-node and -cloud-node-lifecycle to the end of the --controllers flag
			https://github.com/k3s-io/k3s/blob/master/docs/adrs/servicelb-ccm.md
			*/
			It("should find cloud-controller-manager starting with"+
				"\"--cloud-node,--cloud-node-lifecycle,--secure-port=0\" flags ", func() {
				Eventually(func() error {
					match, err := testutil.SearchK3sLog(server, "Running cloud-controller-manager --allocate-node-cidrs=true "+
						"--authentication-kubeconfig=/var/lib/rancher/k3s/server/cred/cloud-controller.kubeconfig "+
						"--authorization-kubeconfig=/var/lib/rancher/k3s/server/cred/cloud-controller.kubeconfig --bind-address=127.0.0.1 "+
						"--cloud-config=/var/lib/rancher/k3s/server/etc/cloud-config.yaml --cloud-provider=k3s --cluster-cidr=10.42.0.0/16 "+
						"--configure-cloud-routes=false --controllers=*,-route,-cloud-node,-cloud-node-lifecycle "+
						"--kubeconfig=/var/lib/rancher/k3s/server/cred/cloud-controller.kubeconfig "+
						"--leader-elect-resource-name=k3s-cloud-controller-manager --node-status-update-frequency=1m0s --profiling=false --secure-port=0")
					if err != nil {
						return err
					}
					if match {
						return nil
					}
					return errors.New("found cloud-controller-manager starting with wrong flags")
				}, "30s", "2s").Should(Succeed())
			})
			It("kills previous server and clean logs", func() {
				Expect(testutil.K3sKillServer(server)).To(Succeed())
			})
			It("start up with no problems and fully disabled cloud controller", func() {
				var err error
				localServerArgs := []string{
					"--cluster-init",
					"--disable-cloud-controller=true",
					"--disable=servicelb",
				}
				server, err = testutil.K3sStartServer(localServerArgs...)
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() error {
					return tests.CheckDefaultDeployments(testutil.DefaultConfig)
				}, "180s", "5s").Should(Succeed())

			})
			It("should not find cloud-controller-manager starting", func() {
				Consistently(func() error {
					match, err := testutil.SearchK3sLog(server, "Running cloud-controller-manager")
					if err != nil {
						return err
					}
					if !match {
						return nil
					}
					return errors.New("found cloud-controller-manager starting")
				}, "100s", "5s").Should(Succeed())
			})
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
