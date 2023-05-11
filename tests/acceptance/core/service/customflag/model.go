package customflag

import (
	"fmt"
	"strconv"
	"strings"
)

var (
	InstallType        InstallTypeValue
	InstallUpgradeFlag MultiValueFlag
	TestCase           TestConfigFlag
)

// InstallTypeValue is a customFlag type that can be used to parse the installation type
type InstallTypeValue struct {
	Version string
	Commit  string
}

// TestConfigFlag TesConfigFlag is a customFlag type that can be used to parse the test case
type TestConfigFlag struct {
	TestFuncName   string
	TestFunc       TestCaseFlagType
	DeployWorkload bool
}

// TestCaseFlagType is a custom customFlag type that can be used to parse the test case
type TestCaseFlagType func(deployWorkload bool)

// MultiValueFlag is a customFlag type that can be used to parse multiple values
type MultiValueFlag []string

// String returns the string representation of the TestConfigFlag
func (t *TestConfigFlag) String() string {
	return fmt.Sprintf("TestFuncName: %s, DeployWorkload: %t", t.TestFuncName, t.DeployWorkload)
}

// Set parses the customFlag value for TestConfigFlag
func (t *TestConfigFlag) Set(value string) error {
	parts := strings.Split(value, ",")

	if len(parts) < 1 {
		return fmt.Errorf("invalid test case customflag format")
	}

	t.TestFuncName = parts[0]
	if len(parts) > 1 {
		deployWorkload, err := strconv.ParseBool(parts[1])
		if err != nil {
			return fmt.Errorf("invalid deploy workload customflag: %v", err)
		}
		t.DeployWorkload = deployWorkload
	}

	return nil
}

// String returns the string representation of the MultiValueFlag
func (m *MultiValueFlag) String() string {
	return strings.Join(*m, ",")
}

// Set parses the customFlag value for MultiValueFlag
func (m *MultiValueFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

// String returns the string representation of the InstallTypeValue
func (it *InstallTypeValue) String() string {
	return fmt.Sprintf("Version: %s, Commit: %s", it.Version, it.Commit)
}

// Set parses the customFlag value for InstallTypeValue
func (it *InstallTypeValue) Set(value string) error {
	parts := strings.Split(value, "=")

	if len(parts) == 2 {
		switch parts[0] {
		case "INSTALL_K3S_VERSION":
			it.Version = parts[1]
		case "INSTALL_K3S_COMMIT":
			it.Commit = parts[1]
		default:
			return fmt.Errorf("invalid install type")
		}
	} else {
		return fmt.Errorf("invalid input format")
	}

	return nil
}
