package template

import (
	g2 "github.com/onsi/ginkgo/v2"
)

// VersionTestTemplate represents a version test scenario with test configurations and commands.
type VersionTestTemplate struct {
	Description     string
	TestCombination *RunCmd
	InstallUpgrade  []string
	TestConfig      *TestConfig
}

// RunCmd represents the command sets to run on host and node.
type RunCmd struct {
	RunOnHost []TestMap
	RunOnNode []TestMap
}

// TestMap represents a single test command with key:value pairs.
type TestMap struct {
	Cmd                  string
	ExpectedValue        string
	ExpectedValueUpgrade string
}

// TestConfig represents the testcase function configuration
type TestConfig struct {
	Name           string
	TestFunc       TestCase
	DeployWorkload bool
}

// TestCase is a custom type representing the test function.
type TestCase func(g g2.GinkgoTestingT, deployWorkload bool)

// TestCaseWrapper wraps a test function and calls it with the given GinkgoTInterface and VersionTestTemplate.
func TestCaseWrapper(g g2.GinkgoTInterface, v VersionTestTemplate) {
	v.TestConfig.TestFunc(g, v.TestConfig.DeployWorkload)
}
