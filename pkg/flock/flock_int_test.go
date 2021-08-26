package flock_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/rancher/k3s/pkg/flock"
)

const lockfile = "/tmp/testlock.test"

var lock int
var _ = Describe("file locks", func() {
	When("a new exclusive lock is created", func() {
		It("starts up with no problems", func() {
			var err error
			lock, err = flock.Acquire(lockfile)
			Expect(err).ToNot(HaveOccurred())
		})
		It("has a write lock on the file", func() {
			Expect(flock.CheckLock(lockfile)).To(BeTrue())
		})
		It("release the lock correctly", func() {
			Expect(flock.Release(lock)).To(Succeed())
			Expect(flock.CheckLock(lockfile)).To(BeFalse())
		})
	})
})

func TestFlock(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Flock Suite", []Reporter{
		reporters.NewJUnitReporter("/tmp/results/junit-flock.xml"),
	})
}
