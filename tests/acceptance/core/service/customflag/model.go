package customflag

import (
	"fmt"
	"strconv"
	"strings"
)

var ServiceFlag FlagConfig
var TestCaseNameFlag StringSlice

type FlagConfig struct {
	InstallType    InstallTypeValueFlag
	InstallUpgrade MultiValueFlag
	TestConfig     TestConfigFlag
	ClusterConfig  ClusterConfigFlag
}

// InstallTypeValueFlag is a customFlag type that can be used to parse the installation type
type InstallTypeValueFlag struct {
	Version []string
	Commit  []string
	Channel string
}

// TestConfigFlag is a customFlag type that can be used to parse the test case
type TestConfigFlag struct {
	TestFuncNames  []string
	TestFuncs      []TestCaseFlag
	DeployWorkload bool
	WorkloadName   string
	Description    string
}

type DestroyFlag bool
type ArchFlag string

// ClusterConfigFlag is a customFlag type that can be used to change some cluster config
type ClusterConfigFlag struct {
	Destroy DestroyFlag
	Arch    ArchFlag
}

// TestCaseFlag is a customFlag type that can be used to parse the test case
type TestCaseFlag func(deployWorkload bool)

// MultiValueFlag is a customFlag type that can be used to parse multiple values
type MultiValueFlag []string

// StringSlice defines a custom flag type for string slice
type StringSlice []string

// String returns the string representation of the StringSlice
func (s *StringSlice) String() string {
	return strings.Join(*s, ",")
}

// Set parses the input string and sets the StringSlice using Set customflag interface
func (s *StringSlice) Set(value string) error {
	*s = strings.Split(value, ",")
	return nil
}

// String returns the string representation of the TestConfigFlag
func (t *TestConfigFlag) String() string {
	return fmt.Sprintf("TestFuncName: %s", t.TestFuncNames)
}

// Set parses the customFlag value for TestConfigFlag
func (t *TestConfigFlag) Set(value string) error {
	t.TestFuncNames = strings.Split(value, ",")
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
func (i *InstallTypeValueFlag) String() string {
	return fmt.Sprintf("Version: %s, Commit: %s", i.Version, i.Commit)
}

// Set parses the customFlag value for InstallTypeValueFlag
func (i *InstallTypeValueFlag) Set(value string) error {
	parts := strings.Split(value, "=")

	for _, part := range parts {
		subParts := strings.Split(part, "=")
		fmt.Println(subParts)
		fmt.Println("sub: ", subParts[len(subParts)-1])

		if len(subParts) != 2 {
			return fmt.Errorf("invalid input format")
		}

		switch parts[0] {
		case "INSTALL_K3S_VERSION":
			i.Version = append(i.Version, subParts[1])
		case "INSTALL_K3S_COMMIT":
			i.Commit = append(i.Commit, subParts[1])
		default:
			return fmt.Errorf("invalid install type: %s", parts[0])
		}
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
