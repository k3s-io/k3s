package template

import (
	"github.com/k3s-io/k3s/tests/acceptance/core/service/customflag"
)

var TestMapTemplate TestMap

// VersionTestTemplate represents a version test scenario with test configurations and commands.
type VersionTestTemplate struct {
	TestCombination *RunCmd
	InstallUpgrade  []string
	TestConfig      *TestConfig
	Description     string
}

// RunCmd represents the command sets to run on host and node.
type RunCmd struct {
	Run []TestMap
}

// TestMap represents a single test command with key:value pairs.
type TestMap struct {
	Cmd                  string
	ExpectedValue        string
	ExpectedValueUpgrade string
}

// TestConfig represents the testcase function configuration
type TestConfig struct {
	TestFunc       []TestCase
	DeployWorkload bool
	WorkloadName   string
}

// TestCase is a custom type representing the test function.
type TestCase func(deployWorkload bool)

// TestCaseWrapper wraps a test function and calls it with the given GinkgoTInterface and VersionTestTemplate.
func TestCaseWrapper(v VersionTestTemplate) {
	for _, testFunc := range v.TestConfig.TestFunc {
		testFunc(v.TestConfig.DeployWorkload)
	}
}

// ConvertToTestCase converts the TestCaseFlag to TestCase
func ConvertToTestCase(testCaseFlags []customflag.TestCaseFlag) []TestCase {
	var testCases []TestCase
	for _, tcf := range testCaseFlags {
		tc := TestCase(tcf)
		testCases = append(testCases, tc)
	}

	return testCases
}
