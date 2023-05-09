package versionbump

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/template"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	// var err error
	flag.StringVar(&util.CmdHost, "cmdHost", "", "Comma separated list of commands to execute on host")
	flag.StringVar(&util.ExpectedValueHost, "expectedValueHost", "", "Comma separated list of expected values for host commands")
	flag.StringVar(&util.CmdNode, "cmdNode", "", "Comma separated list of commands to execute on node")
	flag.StringVar(&util.ExpectedValueNode, "expectedValueNode", "", "Comma separated list of expected values for node commands")
	flag.StringVar(&util.ExpectedValueUpgradedHost, "expectedValueUpgradedHost", "", "Expected value of the command ran on Host after upgrading")
	flag.StringVar(&util.ExpectedValueUpgradedNode, "expectedValueUpgradedNode", "", "Expected value of the command ran on Node after upgrading")
	flag.Var(&util.InstallUpgradeFlag, "installUpgradeFlag", "Install upgrade flag")
	flag.StringVar(&util.Description, "description", "", "Description of the test")
	flag.Var(&util.TestCase, "testCase", "Test case to run")
	flag.BoolVar(&util.TestCase.DeployWorkload, "deployWorkload", false, "Deploy workload flag")
	flag.Parse()

	testFunc, err := template.GetTestCase(util.TestCase.TestFuncName)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if testFunc != nil {
		util.TestCase.TestFunc = util.TestCaseFlagType(testFunc)
	}

	os.Exit(m.Run())
}

func TestVersionTestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Version Test Suite")
}

var _ = AfterSuite(func() {
	g := GinkgoT()
	if *util.Destroy {
		status, err := factory.BuildCluster(g, *util.Destroy)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("cluster destroyed"))
	}
})
