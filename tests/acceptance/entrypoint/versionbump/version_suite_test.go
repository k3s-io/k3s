package versionbump

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/customflag"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/template"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	flag.StringVar(&CmdHost, "cmdHost", "", "Comma separated list of commands to execute on host")
	flag.StringVar(&ExpectedValueHost, "expectedValueHost", "", "Comma separated list of expected values for host commands")
	flag.StringVar(&CmdNode, "cmdNode", "", "Comma separated list of commands to execute on node")
	flag.StringVar(&ExpectedValueNode, "expectedValueNode", "", "Comma separated list of expected values for node commands")
	flag.StringVar(&ExpectedValueUpgradedHost, "expectedValueUpgradedHost", "", "Expected value of the command ran on Host after upgrading")
	flag.StringVar(&ExpectedValueUpgradedNode, "expectedValueUpgradedNode", "", "Expected value of the command ran on Node after upgrading")
	flag.Var(&customflag.InstallUpgradeFlag, "installUpgradeFlag", "Install upgrade customflag")
	flag.StringVar(&Description, "description", "", "Description of the test")
	flag.Var(&customflag.TestCase, "testCase", "Test case to run")
	flag.BoolVar(&customflag.TestCase.DeployWorkload, "deployWorkload", false, "Deploy workload customflag")

	flag.Parse()

	testFunc, err := template.GetTestCase(customflag.TestCase.TestFuncName)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if testFunc != nil {
		customflag.TestCase.TestFunc = customflag.TestCaseFlagType(testFunc)
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
