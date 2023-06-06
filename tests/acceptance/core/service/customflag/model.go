package customflag

import (
	"fmt"
	"strconv"
	"strings"
)

var ServiceFlag FlagConfig

type FlagConfig struct {
	InstallType    InstallTypeValueFlag
	InstallUpgrade MultiValueFlag
	TestCase       TestConfigFlag
	ClusterConfig  ClusterConfigFlag
}

// InstallTypeValueFlag is a customFlag type that can be used to parse the installation type
type InstallTypeValueFlag struct {
	Version string
	Commit  string
}

// TestConfigFlag is a customFlag type that can be used to parse the test case
type TestConfigFlag struct {
	TestFuncName   *string
	TestFunc       TestCaseFlagType
	DeployWorkload bool
}

type DestroyFlag bool
type ArchFlag string

// ClusterConfigFlag is a customFlag type that can be used to change some cluster config
type ClusterConfigFlag struct {
	Destroy DestroyFlag
	Arch    ArchFlag
}

// TestCaseFlagType is a customFlag type that can be used to parse the test case
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

	t.TestFuncName = &parts[0]
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

// String returns the string representation of the InstallTypeValueFlag
func (it *InstallTypeValueFlag) String() string {
	return fmt.Sprintf("Version: %s, Commit: %s", it.Version, it.Commit)
}

// Set parses the customFlag value for InstallTypeValueFlag
func (it *InstallTypeValueFlag) Set(value string) error {
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

// String returns the string representation of the DestroyFlag
func (d *DestroyFlag) String() string {
	return fmt.Sprintf("%v", *d)
}

// Set parses the customFlag value for DestroyFlag
func (d *DestroyFlag) Set(value string) error {
	v, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	*d = DestroyFlag(v)

	return nil
}

// String returns the string representation of the ArchFlag
func (a *ArchFlag) String() string {
	return string(*a)
}

// Set parses the customFlag value for ArchFlag
func (a *ArchFlag) Set(value string) error {
	if value == "arm64" || value == "amd64" {
		*a = ArchFlag(value)
	} else {
		*a = "amd64"
	}

	return nil
}
