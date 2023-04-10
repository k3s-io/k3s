package versionbump

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/template"
	util2 "github.com/k3s-io/k3s/tests/acceptance/shared/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	var err error
	flag.StringVar(&util2.CmdHost, "cmdHost", "", "Comma separated list of commands to execute on host")
	flag.StringVar(&util2.ExpectedValueHost, "expectedValueHost", "", "Comma separated list of expected values for host commands")
	flag.StringVar(&util2.CmdNode, "cmdNode", "", "Comma separated list of commands to execute on node")
	flag.StringVar(&util2.ExpectedValueNode, "expectedValueNode", "", "Comma separated list of expected values for node commands")
	flag.StringVar(&util2.ExpectedValueUpgradedHost, "expectedValueUpgradedHost", "", "Expected value of the command ran on Host after upgrading")
	flag.StringVar(&util2.ExpectedValueUpgradedNode, "expectedValueUpgradedNode", "", "Expected value of the command ran on Node after upgrading")
	flag.Var(&util2.InstallUpgradeFlag, "installUpgradeFlag", "Install upgrade flag")
	flag.StringVar(&util2.Description, "description", "", "Description of the test")
	flag.Var(&util2.TestCase, "testCase", "Test case to run")
	flag.BoolVar(&util2.TestCase.DeployWorkload, "deployWorkload", false, "Deploy workload flag")
	flag.Parse()

	testFunc, err := template.GetTestCase(util2.TestCase.TestFuncName)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if testFunc != nil {
		util2.TestCase.TestFunc = util2.TestCaseFlagType(testFunc)
	}

	os.Exit(m.Run())
}

func TestVersionTestSuite(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Version Test Suite")
}

var _ = AfterSuite(func() {
	g := GinkgoT()
	if *util2.Destroy {
		status, err := factory.BuildCluster(g, *util2.Destroy)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("cluster destroyed"))
	}
})
