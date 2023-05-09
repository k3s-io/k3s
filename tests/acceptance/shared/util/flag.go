package util

import (
	"fmt"
	"strconv"
	"strings"

	// "github.com/k3s-io/k3s/tests/acceptance/service/template"
	// "github.com/k3s-io/k3s/tests/acceptance/testcase"
	g2 "github.com/onsi/ginkgo/v2"
)

// InstallTypeValue is a custom flag type that can be used to parse the installation type
type InstallTypeValue struct {
	Version string
	Commit  string
}

// TestConfigFlag TesConfigFlag is a custom flag type that can be used to parse the test case flag
type TestConfigFlag struct {
	TestFuncName   string
	TestFunc       TestCaseFlagType
	DeployWorkload bool
}

type TestCaseFlagType func(g g2.GinkgoTestingT, deployWorkload bool)

type MultiValueFlag []string

func (t *TestConfigFlag) String() string {
	return fmt.Sprintf("TestFuncName: %s, DeployWorkload: %t", t.TestFuncName, t.DeployWorkload)
}

func (t *TestConfigFlag) Set(value string) error {
	parts := strings.Split(value, ",")

	if len(parts) < 1 {
		return fmt.Errorf("invalid test case flag format")
	}

	t.TestFuncName = parts[0]
	if len(parts) > 1 {
		deployWorkload, err := strconv.ParseBool(parts[1])
		if err != nil {
			return fmt.Errorf("invalid deploy workload flag: %v", err)
		}
		t.DeployWorkload = deployWorkload
	}

	return nil
}

func (m *MultiValueFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *MultiValueFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func (it *InstallTypeValue) String() string {
	return fmt.Sprintf("Version: %s, Commit: %s", it.Version, it.Commit)
}

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
