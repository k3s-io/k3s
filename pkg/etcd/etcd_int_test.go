package etcd_test

import (
	"bufio"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher/k3s/pkg/util/tests"
)

var serverCmd *exec.Cmd
var serverScan *bufio.Scanner
var _ = BeforeSuite(func() {
	var err error
	// if !tests.IsRoot() {
	// 	Fail("User is not root")
	// }
	serverCmd, serverScan, err = tests.K3sCmdAsync("server", "--cluster-init")
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("etcd snapshots", func() {
	When("a new etcd is created", func() {
		It("starts up with no problems", func() {
			// tests.FindStringInCmdAsync(serverScan, "etcd data store connection OK")
			Eventually(func() (string, error) {
				return tests.K3sCmd("kubectl", "get", "pods", "-A")
			}, "90s", "1s").Should(MatchRegexp("kube-system.+coredns.+1\\/1.+Running"))
		})
		It("saves an etcd snapshot", func() {
			Expect(tests.K3sCmd("etcd-snapshot", "save")).
				To(ContainSubstring("Saving current etcd snapshot set to k3s-etcd-snapshots"))
		})
		It("list snapshots", func() {
			Expect(tests.K3sCmd("etcd-snapshot", "ls")).
				To(MatchRegexp(`:///var/lib/rancher/k3s/server/db/snapshots/on-demand`))
		})
		It("deletes a snapshot", func() {
			lsResult, err := tests.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`on-demand[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			snapshotName := reg.FindString(lsResult)
			Expect(tests.K3sCmd("etcd-snapshot", "delete", snapshotName)).
				To(ContainSubstring("Removing the given locally stored etcd snapshot"))
		})
	})
	When("saving a custom name", func() {
		It("starts with no snapshots", func() {
			Expect(tests.K3sCmd("etcd-snapshot", "ls")).To(BeEmpty())
		})
		It("saves an etcd snapshot with a custom name", func() {
			Expect(tests.K3sCmd("etcd-snapshot", "save", "--name", "ALIVEBEEF")).
				To(ContainSubstring("Saving etcd snapshot to /var/lib/rancher/k3s/server/db/snapshots/ALIVEBEEF"))
		})
		It("deletes that snapshot", func() {
			lsResult, err := tests.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`ALIVEBEEF[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			snapshotName := reg.FindString(lsResult)
			Expect(tests.K3sCmd("etcd-snapshot", "delete", snapshotName)).
				To(ContainSubstring("Removing the given locally stored etcd snapshot"))
		})
	})
	When("using etcd snapshot prune", func() {
		It("starts with no snapshots", func() {
			Expect(tests.K3sCmd("etcd-snapshot", "ls")).To(BeEmpty())
		})
		It("saves 3 different snapshots", func() {
			Expect(tests.K3sCmd("etcd-snapshot", "save", "-name", "PRUNE_TEST")).
				To(ContainSubstring("Saving current etcd snapshot set to k3s-etcd-snapshots"))
			time.Sleep(1 * time.Second)
			Expect(tests.K3sCmd("etcd-snapshot", "save", "-name", "PRUNE_TEST")).
				To(ContainSubstring("Saving current etcd snapshot set to k3s-etcd-snapshots"))
			time.Sleep(1 * time.Second)
			Expect(tests.K3sCmd("etcd-snapshot", "save", "-name", "PRUNE_TEST")).
				To(ContainSubstring("Saving current etcd snapshot set to k3s-etcd-snapshots"))
			time.Sleep(1 * time.Second)
		})
		It("lists all 3 snapshots", func() {
			lsResult, err := tests.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			sepLines := strings.FieldsFunc(lsResult, func(c rune) bool {
				return c == '\n'
			})
			Expect(lsResult).To(MatchRegexp(`:///var/lib/rancher/k3s/server/db/snapshots/PRUNE_TEST`))
			Expect(sepLines).To(HaveLen(3))
		})
		It("prunes snapshots down to 2", func() {
			Expect(tests.K3sCmd("etcd-snapshot", "prune", "--snapshot-retention", "2", "--name", "PRUNE_TEST")).
				To(BeEmpty())
			lsResult, err := tests.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			sepLines := strings.FieldsFunc(lsResult, func(c rune) bool {
				return c == '\n'
			})
			Expect(lsResult).To(MatchRegexp(`:///var/lib/rancher/k3s/server/db/snapshots/PRUNE_TEST`))
			Expect(sepLines).To(HaveLen(2))
		})
		It("cleans up remaining snapshots", func() {
			lsResult, err := tests.K3sCmd("etcd-snapshot", "ls")
			Expect(err).ToNot(HaveOccurred())
			reg, err := regexp.Compile(`PRUNE_TEST[^\s]+`)
			Expect(err).ToNot(HaveOccurred())
			for _, snapshotName := range reg.FindAllString(lsResult, -1) {
				Expect(tests.K3sCmd("etcd-snapshot", "delete", snapshotName)).
					To(ContainSubstring("Removing the given locally stored etcd snapshot"))
			}
		})
	})
})

var _ = AfterSuite(func() {
	if serverCmd != nil {
		Expect(serverCmd.Process.Kill()).To(Succeed())
	}
})

func TestIntegration_Etcd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Etcd Suite")
}
