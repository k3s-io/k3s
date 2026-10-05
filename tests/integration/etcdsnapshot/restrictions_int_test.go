package snapshot_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	testutil "github.com/k3s-io/k3s/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("etcd snapshot restrictions", Ordered, func() {
	var configServerArgs []string
	var restrictedServerDir string

	BeforeEach(func() {
		if !testutil.IsExistingServer() && server != nil {
			Expect(testutil.K3sKillServer(server)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
			server = nil
		}
		var err error
		restrictedServerDir, err = os.MkdirTemp("", "k3s-test-restricted-dir-")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if !testutil.IsExistingServer() && server != nil {
			Expect(testutil.K3sKillServer(server)).To(Succeed())
			Expect(testutil.K3sCleanup(-1, "")).To(Succeed())
			server = nil
		}
		if restrictedServerDir != "" {
			os.RemoveAll(restrictedServerDir)
			restrictedServerDir = ""
		}
	})

	When("a server is started with snapshot-dir and s3-bucket restrictions", func() {
		It("ignores restricted overrides and warns to stderr", func() {
			configServerArgs = []string{
				"--cluster-init",
				"--etcd-snapshot-restrictions=snapshot-dir,s3-bucket",
				"--etcd-snapshot-dir=" + restrictedServerDir,
			}
			var err error
			server, err = testutil.K3sStartServer(configServerArgs...)
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

			matches, err := filepath.Glob(filepath.Join(restrictedServerDir, "on-demand*"))
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
			configServerArgs = []string{
				"--cluster-init",
				"--etcd-snapshot-restrictions=all",
				"--etcd-snapshot-dir=" + restrictedServerDir,
			}
			var err error
			server, err = testutil.K3sStartServer(configServerArgs...)
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

			matches, err := filepath.Glob(filepath.Join(restrictedServerDir, "on-demand*"))
			Expect(err).ToNot(HaveOccurred())
			Expect(matches).ToNot(BeEmpty(), "snapshot was not saved in the configured restricted directory")
		})
	})

	When("deleting a snapshot securely", func() {
		It("prevents path traversal local delete", func() {
			configServerArgs = []string{
				"--cluster-init",
				"--etcd-snapshot-restrictions=snapshot-dir",
				"--etcd-snapshot-dir=" + restrictedServerDir,
			}
			var err error
			server, err = testutil.K3sStartServer(configServerArgs...)
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
			configServerArgs = []string{
				"--cluster-init",
				"--etcd-snapshot-dir=" + restrictedServerDir,
			}
			var err error
			server, err = testutil.K3sStartServer(configServerArgs...)
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
