package snapshot_test

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tests "github.com/k3s-io/k3s/tests"
	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var server *testutil.K3sServer
var serverArgs = []string{"--cluster-init"}
var testLock int
var populatedTestSnapshotDir string
var emptyTestSnapshotDir string
var etcdSnapshotFilePattern = "test-snapshot"
var etcdSnapshotRetention = 1

var _ = BeforeSuite(func() {
	if !testutil.IsExistingServer() {
		var err error
		testLock, err = testutil.K3sTestLock()
		Expect(err).ToNot(HaveOccurred())
		server, err = testutil.K3sStartServer(serverArgs...)
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = Describe("etcd snapshots", Ordered, func() {
	BeforeEach(func() {
		if testutil.IsExistingServer() && !testutil.ServerArgsPresent(serverArgs) {
			Skip("Test needs k3s server with: " + strings.Join(serverArgs, " "))
		}
	})
	When("a new etcd is created", func() {
		It("starts up with no problems", func() {
			Eventually(func() error {
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "180s", "10s").Should(Succeed())
		})
		It("saves an etcd snapshot", func() {
			Expect(testutil.K3sCmd("etcd-snapshot", "save")).
				To(ContainSubstring("saved"))
		})
		It("list snapshots", func() {
			Expect(testutil.K3sCmd("etcd-snapshot", "ls")).
				To(MatchRegexp(`:///var/lib/rancher/k3s/server/db/snapshots/on-demand`))
		})
		It("deletes a snapshot", func() {
			lsResult, err := testutil.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`(?m)^on-demand[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			snapshotName := reg.FindString(lsResult)
			Expect(testutil.K3sCmd("etcd-snapshot", "delete", snapshotName)).
				To(ContainSubstring("Snapshot " + snapshotName + " deleted"))
		})
	})
	When("saving a custom name", func() {
		It("saves an etcd snapshot with a custom name", func() {
			Expect(testutil.K3sCmd("etcd-snapshot", "save --name ALIVEBEEF")).
				To(ContainSubstring("Snapshot ALIVEBEEF-"))
		})
		It("deletes that snapshot", func() {
			lsResult, err := testutil.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`(?m)^ALIVEBEEF[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			snapshotName := reg.FindString(lsResult)
			Expect(testutil.K3sCmd("etcd-snapshot", "delete", snapshotName)).
				To(ContainSubstring("Snapshot " + snapshotName + " deleted"))
		})
	})
	When("using etcd snapshot prune", func() {
		It("saves 3 different snapshots", func() {
			Expect(testutil.K3sCmd("etcd-snapshot", "save -name PRUNE_TEST")).
				To(ContainSubstring("saved"))
			time.Sleep(1 * time.Second)
			Expect(testutil.K3sCmd("etcd-snapshot", "save -name PRUNE_TEST")).
				To(ContainSubstring("saved"))
			time.Sleep(1 * time.Second)
			Expect(testutil.K3sCmd("etcd-snapshot", "save -name PRUNE_TEST")).
				To(ContainSubstring("saved"))
			time.Sleep(1 * time.Second)
		})
		It("lists all 3 snapshots", func() {
			lsResult, err := testutil.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`(?m):///var/lib/rancher/k3s/server/db/snapshots/PRUNE_TEST`)
			Expect(err).ToNot(HaveOccurred())
			sepLines := reg.FindAllString(lsResult, -1)
			Expect(sepLines).To(HaveLen(3))
		})
		It("prunes snapshots down to 2", func() {
			Expect(testutil.K3sCmd("etcd-snapshot", "prune --snapshot-retention 2 --name PRUNE_TEST")).
				To(ContainSubstring(" deleted."))
			lsResult, err := testutil.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`(?m):///var/lib/rancher/k3s/server/db/snapshots/PRUNE_TEST`)
			Expect(err).ToNot(HaveOccurred())
			sepLines := reg.FindAllString(lsResult, -1)
			Expect(sepLines).To(HaveLen(2))
		})
		It("cleans up remaining snapshots", func() {
			lsResult, err := testutil.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`(?m)^PRUNE_TEST[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			for _, snapshotName := range reg.FindAllString(lsResult, -1) {
				Expect(testutil.K3sCmd("etcd-snapshot", "delete", snapshotName)).
					To(ContainSubstring("Snapshot " + snapshotName + " deleted"))
			}
		})
	})
	When("a new etcd is created with server start flags", func() {
		It("kills previous server and start up with no problems", func() {
			var err error
			Expect(testutil.K3sKillServer(server)).To(Succeed())
			localServerArgs := []string{"--cluster-init",
				"--etcd-snapshot-name", etcdSnapshotFilePattern,
				"--etcd-snapshot-dir", populatedTestSnapshotDir,
				"--etcd-snapshot-retention", fmt.Sprint(etcdSnapshotRetention),
				"--etcd-snapshot-schedule-cron", `* * * * *`,
				"--etcd-snapshot-compress"}
			server, err = testutil.K3sStartServer(localServerArgs...)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "180s", "5s").Should(Succeed())

		})
		It("saves an etcd snapshot with specified name and it should be no more than 1 compressed file", func() {

			Eventually(func() (int, error) {
				matches, err := filepath.Glob(filepath.Join(populatedTestSnapshotDir, fmt.Sprintf("%s%s%s", "*", etcdSnapshotFilePattern, "*.zip")))
				return len(matches), err
			}, "180s", "30s").Should(Equal(etcdSnapshotRetention))
			Consistently(func() (int, error) {
				matches, err := filepath.Glob(filepath.Join(populatedTestSnapshotDir, fmt.Sprintf("%s%s%s", "*", etcdSnapshotFilePattern, "*.zip")))
				return len(matches), err
			}, "120s", "30s").Should(Equal(etcdSnapshotRetention))
		})
		It("kills previous server and start up with no problems and disabled snapshots", func() {

			var err error
			Expect(testutil.K3sKillServer(server)).To(Succeed())
			localServerArgs := []string{"--cluster-init",
				"--etcd-snapshot-dir", emptyTestSnapshotDir,
				"--etcd-snapshot-schedule-cron", `* * * * *`,
				"--etcd-disable-snapshots"}
			server, err = testutil.K3sStartServer(localServerArgs...)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(testutil.DefaultConfig)
			}, "180s", "5s").Should(Succeed())

		})
		It("should not save any snapshot", func() {
			Consistently(func() error {
				matches, err := filepath.Glob(filepath.Join(emptyTestSnapshotDir, "*"))
				if matches != nil || err != nil {
					return fmt.Errorf("something went wrong: err != nil (%v) or matches != nil (%v)", err, matches)
				}
				return nil
			}, "180s", "60s").Should(Succeed())
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
	populatedTestSnapshotDir = t.TempDir()
	emptyTestSnapshotDir = t.TempDir()
	RunSpecs(t, "Etcd Snapshot Suite")
}
