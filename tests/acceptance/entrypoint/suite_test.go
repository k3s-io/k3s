package entrypoint

import (
	"testing"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Acceptance Test Suite")
}

var _ = BeforeSuite(func() {
	ginkgoTInterface := GinkgoT()
	_, err := factory.BuildCluster(ginkgoTInterface, *util.Destroy)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	g := GinkgoT()
	if *util.Destroy {
		_, err := factory.BuildCluster(g, true)
		Expect(err).NotTo(HaveOccurred())
	}
})
