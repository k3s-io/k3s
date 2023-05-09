package upgradecluster

import (
	"flag"
	"testing"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestClusterUpgradeSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()

	RunSpecs(t, "Upgrade Cluster Test Suite")
}

var _ = AfterSuite(func() {
	g := GinkgoT()
	if *util.Destroy {
		status, err := factory.BuildCluster(g, *util.Destroy)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("cluster destroyed"))
	}
})
