package etcd_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

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

var _ = Describe("etcd snapshots", func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(serverArgs) {
			Skip("Test needs k3s server with: " + strings.Join(serverArgs, " "))
		}
	})
	When("a new etcd is created", func() {
		It("starts up with no problems", func() {
			Eventually(func() (string, error) {
				return testutil.K3sCmd("kubectl", "get", "pods", "-A")
			}, "90s", "1s").Should(MatchRegexp("kube-system.+coredns.+1\\/1.+Running"))
		})
		It("saves an etcd snapshot", func() {
			Expect(testutil.K3sCmd("etcd-snapshot", "save")).
				To(ContainSubstring("Saving current etcd snapshot set to k3s-etcd-snapshots"))
		})
		It("list snapshots", func() {
			Expect(testutil.K3sCmd("etcd-snapshot", "ls")).
				To(MatchRegexp(`:///var/lib/rancher/k3s/server/db/snapshots/on-demand`))
		})
		It("deletes a snapshot", func() {
			lsResult, err := testutil.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`on-demand[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			snapshotName := reg.FindString(lsResult)
			Expect(testutil.K3sCmd("etcd-snapshot", "delete", snapshotName)).
				To(ContainSubstring("Removing the given locally stored etcd snapshot"))
		})
	})
	When("saving a custom name", func() {
		It("saves an etcd snapshot with a custom name", func() {
			Expect(testutil.K3sCmd("etcd-snapshot", "save", "--name", "ALIVEBEEF")).
				To(ContainSubstring("Saving etcd snapshot to /var/lib/rancher/k3s/server/db/snapshots/ALIVEBEEF"))
		})
		It("deletes that snapshot", func() {
			lsResult, err := testutil.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`ALIVEBEEF[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			snapshotName := reg.FindString(lsResult)
			Expect(testutil.K3sCmd("etcd-snapshot", "delete", snapshotName)).
				To(ContainSubstring("Removing the given locally stored etcd snapshot"))
		})
	})
	When("using etcd snapshot prune", func() {
		It("saves 3 different snapshots", func() {
			Expect(testutil.K3sCmd("etcd-snapshot", "save", "-name", "PRUNE_TEST")).
				To(ContainSubstring("Saving current etcd snapshot set to k3s-etcd-snapshots"))
			time.Sleep(1 * time.Second)
			Expect(testutil.K3sCmd("etcd-snapshot", "save", "-name", "PRUNE_TEST")).
				To(ContainSubstring("Saving current etcd snapshot set to k3s-etcd-snapshots"))
			time.Sleep(1 * time.Second)
			Expect(testutil.K3sCmd("etcd-snapshot", "save", "-name", "PRUNE_TEST")).
				To(ContainSubstring("Saving current etcd snapshot set to k3s-etcd-snapshots"))
			time.Sleep(1 * time.Second)
		})
		It("lists all 3 snapshots", func() {
			lsResult, err := testutil.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`:///var/lib/rancher/k3s/server/db/snapshots/PRUNE_TEST`)
			Expect(err).ToNot(HaveOccurred())
			sepLines := reg.FindAllString(lsResult, -1)
			Expect(sepLines).To(HaveLen(3))
		})
		It("prunes snapshots down to 2", func() {
			Expect(testutil.K3sCmd("etcd-snapshot", "prune", "--snapshot-retention", "2", "--name", "PRUNE_TEST")).
				To(BeEmpty())
			lsResult, err := testutil.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`:///var/lib/rancher/k3s/server/db/snapshots/PRUNE_TEST`)
			Expect(err).ToNot(HaveOccurred())
			sepLines := reg.FindAllString(lsResult, -1)
			Expect(sepLines).To(HaveLen(2))
		})
		It("cleans up remaining snapshots", func() {
			lsResult, err := testutil.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`PRUNE_TEST[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			for _, snapshotName := range reg.FindAllString(lsResult, -1) {
				Expect(testutil.K3sCmd("etcd-snapshot", "delete", snapshotName)).
					To(ContainSubstring("Removing the given locally stored etcd snapshot"))
			}
		})
	})
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		Expect(testutil.K3sKillServer(server)).To(Succeed())
	}
})

func Test_IntegrationEtcd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Etcd Suite", []Reporter{
		reporters.NewJUnitReporter("/tmp/results/junit-etcd.xml"),
	})
}
