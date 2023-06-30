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
	flag.StringVar(&template.TestMapFlag.Cmd, "cmd", "", "Comma separated list of commands to execute")
	flag.StringVar(&template.TestMapFlag.ExpectedValue, "expectedValue", "", "Comma separated list of expected values for commands")
	flag.StringVar(&template.TestMapFlag.ExpectedValueUpgrade, "expectedValueUpgrade", "", "Expected value of the command ran after upgrading")
	flag.Var(&customflag.ServiceFlag.InstallUpgrade, "installVersionOrCommit", "Install upgrade customflag for version bump")
	flag.StringVar(&template.TestMapFlag.Description, "description", "", "Description of the test")
	flag.Var(&customflag.ServiceFlag.TestCase, "testCase", "Test case to run")
	flag.BoolVar(&customflag.ServiceFlag.TestCase.DeployWorkload, "deployWorkload", false, "Deploy workload customflag")
	flag.Var(&customflag.ServiceFlag.ClusterConfig.Destroy, "destroy", "Destroy cluster after test")
	flag.Var(&customflag.ServiceFlag.ClusterConfig.Arch, "arch", "Architecture type")

	flag.Parse()

	testFunc, err := template.AddTestCase(*customflag.ServiceFlag.TestCase.TestFuncName)
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
