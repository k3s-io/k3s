package snapshot_test

import (
	"fmt"
	"os"
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
var restrictedTestSnapshotDir string
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
		It("writes the compressed snapshot with non-world-readable permissions", func() {
			matches, err := filepath.Glob(filepath.Join(populatedTestSnapshotDir, fmt.Sprintf("%s%s%s", "*", etcdSnapshotFilePattern, "*.zip")))
			Expect(err).ToNot(HaveOccurred())
			Expect(matches).ToNot(BeEmpty())
			for _, match := range matches {
				info, err := os.Stat(match)
				Expect(err).ToNot(HaveOccurred())
				Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
			}
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

	When("a server is started with snapshot-dir and s3-bucket restrictions", func() {
		It("ignores restricted overrides and warns to stderr", func() {
			localServerArgs := []string{
				"--cluster-init",
				"--etcd-snapshot-restrictions=snapshot-dir,s3-bucket",
				"--etcd-snapshot-dir=" + restrictedTestSnapshotDir,
			}
			var err error
			Expect(testutil.K3sKillServer(server)).To(Succeed())
			server, err = testutil.K3sStartServer(localServerArgs...)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() string {
				res, _ := testutil.K3sCmd("kubectl", "get", "nodes")
				return res
			}, "180s", "5s").Should(ContainSubstring("Ready"))

			maliciousDir := filepath.Join(os.TempDir(), "k3s-malicious")
			os.MkdirAll(maliciousDir, 0700)
			defer os.RemoveAll(maliciousDir)

			res, err := testutil.K3sCmd("etcd-snapshot", "save", "--etcd-snapshot-dir="+maliciousDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(ContainSubstring("saved"))
			expectRestrictionWarning(res, "snapshot-dir")

			matches, err := filepath.Glob(filepath.Join(restrictedTestSnapshotDir, "on-demand*"))
			Expect(err).ToNot(HaveOccurred())
			Expect(matches).ToNot(BeEmpty(), "snapshot was not saved in the configured restricted directory")

			maliciousMatches, err := filepath.Glob(filepath.Join(maliciousDir, "on-demand*"))
			Expect(err).ToNot(HaveOccurred())
			Expect(maliciousMatches).To(BeEmpty(), "snapshot escaped to the malicious directory")

			// The server has no S3 configured, so a restricted S3 target has nothing to be pinned to:
			// the S3 section is dropped and the snapshot saved locally, instead of failing to reach S3.
			res, err = testutil.K3sCmd("etcd-snapshot", "save", "--s3", "--s3-bucket=malicious-bucket")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(ContainSubstring("saved"))
			expectRestrictionWarning(res, "s3-bucket")
		})
	})

	When("a server is started with all restrictions and config.yaml", func() {
		It("drops all overrides from config.yaml and emits warnings", func() {
			localServerArgs := []string{
				"--cluster-init",
				"--etcd-snapshot-restrictions=all",
				"--etcd-snapshot-dir=" + restrictedTestSnapshotDir,
			}
			var err error
			Expect(testutil.K3sKillServer(server)).To(Succeed())
			server, err = testutil.K3sStartServer(localServerArgs...)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() string {
				res, _ := testutil.K3sCmd("kubectl", "get", "nodes")
				return res
			}, "180s", "5s").Should(ContainSubstring("Ready"))

			maliciousDir := filepath.Join(os.TempDir(), "k3s-malicious-all")
			os.MkdirAll(maliciousDir, 0700)
			defer os.RemoveAll(maliciousDir)

			configPath := filepath.Join(os.TempDir(), "k3s-test-config.yaml")
			err = os.WriteFile(configPath, []byte(fmt.Sprintf("etcd-snapshot-dir: %s\netcd-s3: true\netcd-s3-bucket: bad-bucket\netcd-s3-folder: bad-folder\n", maliciousDir)), 0644)
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(configPath)

			res, err := testutil.K3sCmd("etcd-snapshot", "save", "--config", configPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(ContainSubstring("saved"))
			expectRestrictionWarning(res, "snapshot-dir", "s3-bucket", "s3-folder")

			maliciousMatches, err := filepath.Glob(filepath.Join(maliciousDir, "on-demand*"))
			Expect(err).ToNot(HaveOccurred())
			Expect(maliciousMatches).To(BeEmpty(), "snapshot escaped to the directory set in config.yaml")

			matches, err := filepath.Glob(filepath.Join(restrictedTestSnapshotDir, "on-demand*"))
			Expect(err).ToNot(HaveOccurred())
			Expect(matches).ToNot(BeEmpty(), "snapshot was not saved in the configured restricted directory")
		})
	})

	When("deleting a snapshot securely", func() {
		It("prevents path traversal local delete", func() {
			localServerArgs := []string{
				"--cluster-init",
				"--etcd-snapshot-restrictions=snapshot-dir",
				"--etcd-snapshot-dir=" + restrictedTestSnapshotDir,
			}
			var err error
			Expect(testutil.K3sKillServer(server)).To(Succeed())
			server, err = testutil.K3sStartServer(localServerArgs...)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() string {
				res, _ := testutil.K3sCmd("kubectl", "get", "nodes")
				return res
			}, "180s", "5s").Should(ContainSubstring("Ready"))

			res, err := testutil.K3sCmd("etcd-snapshot", "save")
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(ContainSubstring("saved"))

			res, err = testutil.K3sCmd("etcd-snapshot", "delete", "../../../etc/passwd")
			Expect(err).To(HaveOccurred())
			Expect(res).To(ContainSubstring("invalid snapshot name"))

			lsResult, err := testutil.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			lines := strings.Split(lsResult, "\n")
			Expect(len(lines)).To(BeNumerically(">", 1))

			snapshotName := ""
			for _, line := range lines {
				if strings.HasPrefix(line, "on-demand") {
					snapshotName = strings.Fields(line)[0]
					break
				}
			}
			Expect(snapshotName).ToNot(BeEmpty())

			Expect(testutil.K3sCmd("etcd-snapshot", "delete", snapshotName)).
				To(ContainSubstring("Snapshot " + snapshotName + " deleted"))
		})
	})

	When("a server is started without restrictions", func() {
		It("respects the override", func() {
			localServerArgs := []string{
				"--cluster-init",
				"--etcd-snapshot-dir=" + restrictedTestSnapshotDir,
			}
			var err error
			Expect(testutil.K3sKillServer(server)).To(Succeed())
			server, err = testutil.K3sStartServer(localServerArgs...)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() string {
				res, _ := testutil.K3sCmd("kubectl", "get", "nodes")
				return res
			}, "180s", "5s").Should(ContainSubstring("Ready"))

			customDir := filepath.Join(os.TempDir(), "k3s-custom")
			os.MkdirAll(customDir, 0700)
			defer os.RemoveAll(customDir)

			res, err := testutil.K3sCmd("etcd-snapshot", "save", "--etcd-snapshot-dir="+customDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(ContainSubstring("saved"))
			Expect(res).ToNot(ContainSubstring("override ignored"))

			matches, err := filepath.Glob(filepath.Join(customDir, "on-demand*"))
			Expect(err).ToNot(HaveOccurred())
			Expect(matches).ToNot(BeEmpty(), "snapshot was not saved in the overridden directory")
		})
	})
})

// expectRestrictionWarning asserts that the CLI output contains a server-side snapshot
// restriction warning naming each of the given settings, whether the server reported a single
// ignored option or a list of them.
func expectRestrictionWarning(output string, settings ...string) {
	GinkgoHelper()

	var warning string
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "override ignored by server-side snapshot restrictions") ||
			strings.Contains(line, "restricted snapshot options were ignored") {
			warning = line
			break
		}
	}
	Expect(warning).ToNot(BeEmpty(), "no snapshot restriction warning found in output:\n%s", output)
	for _, setting := range settings {
		Expect(warning).To(ContainSubstring(setting))
	}
	Expect(warning).To(ContainSubstring("Using the server-configured"))
}

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if !testutil.IsExistingServer() {
		if failed {
			testutil.K3sSaveLog(server, false)
			testutil.K3sCopyPodLogs(server)
			testutil.K3sDumpResources(server, "node", "pod", "pvc", "pv")
		}
		Expect(testutil.K3sKillServer(server)).To(Succeed())
		Expect(testutil.K3sCleanup(testLock, "")).To(Succeed())
	}
})

func Test_IntegrationEtcdSnapshot(t *testing.T) {
	RegisterFailHandler(Fail)
	populatedTestSnapshotDir = t.TempDir()
	emptyTestSnapshotDir = t.TempDir()
	restrictedTestSnapshotDir = t.TempDir()
	RunSpecs(t, "Etcd Snapshot Suite")
}
