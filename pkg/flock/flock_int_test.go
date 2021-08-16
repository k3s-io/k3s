package flock_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher/k3s/pkg/flock"
)

var lock int
var _ = Describe("file locks", func() {
	When("a new exclusive lock is created", func() {
		It("starts up with no problems", func() {
			var err error
			lock, err = flock.Acquire("/tmp/testlock.test")
			Expect(err).ToNot(HaveOccurred())
		})
		It("has a write lock on the file", func() {
			Expect(flock.CheckLock("/tmp/testlock.test")).To(BeTrue())
		})
		It("release the lock correctly", func() {
			Expect(flock.Release(lock)).To(Succeed())
			Expect(flock.CheckLock("/tmp/testlock.test")).To(BeFalse())
		})
	})
})

func TestFlock(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flock Suite")
}
