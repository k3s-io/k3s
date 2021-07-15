package etcd_test

import (
	"bufio"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher/k3s/pkg/util/tests"
)

var serverCmd *exec.Cmd
var serverScan *bufio.Scanner
var _ = BeforeSuite(func() {
	var err error
	serverCmd, serverScan, err = tests.K3sCmdAsync("server", "--cluster-init")
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("etcd snapshots", func() {
	When("a new etcd is created", func() {
		It("starts up with no problems", func() {
			tests.FindStringInCmdAsync(serverScan, "etcd data store connection OK")
			Eventually(func() (string, error) {
				return tests.K3sCmd("kubectl", "get", "pods", "-A")
			}, "10s", "2s").Should(MatchRegexp("kube-system.+coredns.+Running"))
		})
		It("saves an etcd snapshot", func() {
			result, err := tests.K3sCmd("etcd-snapshot", "save")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Saving current etcd snapshot set to k3s-etcd-snapshots"))
		})
		// It("saves an etcd snapshot", func() {
		// 	result, err := tests.K3sApp("etcd-snapshot", "save")
		// 	Expect(err).NotTo(HaveOccurred())
		// 	Expect(result.AllEntries()).To(ContainElement("Saving current etcd"))
		// })
		It("list snapshots", func() {
			result, err := tests.K3sCmd("etcd-snapshot", "ls")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(MatchRegexp(`:///var/lib/rancher/k3s/server/db/snapshots/on-demand`))
		})
		It("deletes a snapshot", func() {
			lsResult, err := tests.K3sCmd("etcd-snapshot", "ls")
			Expect(err).NotTo(HaveOccurred())
			reg, err := regexp.Compile(`on-demand[^\s]+`)
			Expect(err).NotTo(HaveOccurred())
			snapshotName := reg.FindString(lsResult)
			delResult, err := tests.K3sCmd("etcd-snapshot", "delete", snapshotName)
			Expect(err).NotTo(HaveOccurred())
			Expect(delResult).To(ContainSubstring("Removing the given locally stored etcd snapshot"))
		})
	})
	When("saving a custom name", func() {
		It("starts with no snapshots", func() {
			result, err := tests.K3sCmd("etcd-snapshot", "ls")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
		It("saves an etcd snapshot with a custom name", func() {
			result, err := tests.K3sCmd("etcd-snapshot", "save", "--name", "ALIVEBEEF")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Saving etcd snapshot to /var/lib/rancher/k3s/server/db/snapshots/ALIVEBEEF"))
		})
		It("deletes that snapshot", func() {
			lsResult, err := tests.K3sCmd("etcd-snapshot", "ls")
			Expect(err).NotTo(HaveOccurred())
			reg, err := regexp.Compile(`ALIVEBEEF[^\s]+`)
			Expect(err).NotTo(HaveOccurred())
			snapshotName := reg.FindString(lsResult)
			delResult, err := tests.K3sCmd("etcd-snapshot", "delete", snapshotName)
			Expect(err).NotTo(HaveOccurred())
			Expect(delResult).To(ContainSubstring("Removing the given locally stored etcd snapshot"))
		})
	})
	When("using etcd snapshot prune", func() {
		It("starts with no snapshots", func() {
			result, err := tests.K3sCmd("etcd-snapshot", "ls")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
		It("saves 3 different snapshots, hardcoded with the default CLI name", func() {
			result, err := tests.K3sCmd("etcd-snapshot", "save", "--")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Saving current etcd snapshot set to k3s-etcd-snapshots"))
			result, err = tests.K3sCmd("etcd-snapshot", "save")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Saving current etcd snapshot set to k3s-etcd-snapshots"))
			result, err = tests.K3sCmd("etcd-snapshot", "save")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ContainSubstring("Saving current etcd snapshot set to k3s-etcd-snapshots"))
		})
		It("lists all 3 snapshots", func() {
			lsResult, err := tests.K3sCmd("etcd-snapshot", "ls")
			Expect(err).NotTo(HaveOccurred())
			sepLines := strings.FieldsFunc(lsResult, func(c rune) bool {
				return c == '\n'
			})
			Expect(lsResult).To(MatchRegexp(`:///var/lib/rancher/k3s/server/db/snapshots/on-demand`))
			Expect(sepLines).To(HaveLen(3))
			reg, _ := regexp.Compile(`on-demand[^\s]+`)
			// Cleanup all test snapshots
			for _, snap := range reg.FindAllString(lsResult, -1) {
				delResult, err := tests.K3sCmd("etcd-snapshot", "delete", snap)
				Expect(err).NotTo(HaveOccurred())
				Expect(delResult).To(ContainSubstring("Removing the given locally stored etcd snapshot"))
			}
		})
	})
})

var _ = AfterSuite(func() {
	serverCmd.Process.Kill()
})

func TestIntegration_Etcd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Etcd Suite")
}
