package upgradecluster

import (
	"flag"
	"os"
	"testing"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/customflag"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {

	flag.Var(&customflag.InstallType, "installtype", "Upgrade to run with type=value,"+
		"INSTALL_K3S_VERSION=v1.26.2+k3s1 or INSTALL_K3S_COMMIT=1823dsad7129873192873129asd")

	flag.Parse()

	os.Exit(m.Run())
}

func TestClusterUpgradeSuite(t *testing.T) {
	RegisterFailHandler(Fail)

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
