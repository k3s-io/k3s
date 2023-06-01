package versionbump

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/customflag"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	flag.StringVar(&template.TestMapFlag.CmdHost, "cmdHost", "", "Comma separated list of commands to execute on host")
	flag.StringVar(&template.TestMapFlag.ExpectedValueHost, "expectedValueHost", "", "Comma separated list of expected values for host commands")
	flag.StringVar(&template.TestMapFlag.CmdNode, "cmdNode", "", "Comma separated list of commands to execute on node")
	flag.StringVar(&template.TestMapFlag.ExpectedValueNode, "expectedValueNode", "", "Comma separated list of expected values for node commands")
	flag.StringVar(&template.TestMapFlag.ExpectedValueUpgradedHost, "expectedValueUpgradedHost", "", "Expected value of the command ran on Host after upgrading")
	flag.StringVar(&template.TestMapFlag.ExpectedValueUpgradedNode, "expectedValueUpgradedNode", "", "Expected value of the command ran on Node after upgrading")
	flag.Var(&customflag.ServiceFlag.InstallUpgrade, "installUpgradeFlag", "Install upgrade customflag")
	flag.StringVar(&template.TestMapFlag.Description, "description", "", "Description of the test")
	flag.Var(&customflag.ServiceFlag.TestCase, "testCase", "Test case to run")
	flag.BoolVar(&customflag.ServiceFlag.TestCase.DeployWorkload, "deployWorkload", false, "Deploy workload customflag")
	flag.Var(&customflag.ServiceFlag.ClusterConfig.Destroy, "destroy", "Destroy cluster after test")
	flag.Var(&customflag.ServiceFlag.ClusterConfig.Arch, "arch", "Architecture type")

	flag.Parse()

	testFunc, err := template.AddTestCase(customflag.ServiceFlag.TestCase.TestFuncName)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if testFunc != nil {
		customflag.ServiceFlag.TestCase.TestFunc = customflag.TestCaseFlagType(testFunc)
	}
	os.Exit(m.Run())
}

func TestVersionTestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Version Test Suite")
}

var _ = AfterSuite(func() {
	g := GinkgoT()
	if customflag.ServiceFlag.ClusterConfig.Destroy {
		status, err := factory.DestroyCluster(g)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("cluster destroyed"))
	}
})
