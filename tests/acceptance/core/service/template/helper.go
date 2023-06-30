package template

import (
	"fmt"
	"strings"
	"sync"

	"github.com/k3s-io/k3s/tests/acceptance/core/testcase"
	"github.com/k3s-io/k3s/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
)

// upgradeVersion upgrades the version of k3s and update the expected value
func upgradeVersion(template VersionTestTemplate, version string) error {
	err := testcase.TestUpgradeClusterManually(version)
	if err != nil {
		return err
	}

	for i := range template.TestCombination.Run {
		template.TestCombination.Run[i].ExpectedValue =
			template.TestCombination.Run[i].ExpectedValueUpgrade
	}

	return nil
}

// checkVersion checks the version of k3s and processes tests
func checkVersion(v VersionTestTemplate) error {
	ips, err := getIPs()
	if err != nil {
		GinkgoT().Errorf("Failed to get IPs: %s", err)
	}

	var wg sync.WaitGroup
	errorChanList := make(
		chan error,
		len(ips)*(len(v.TestCombination.Run)),
	)

	processTestCombination(errorChanList, &wg, ips, *v.TestCombination)

	wg.Wait()
	close(errorChanList)

	for errorChan := range errorChanList {
		if errorChan != nil {
			return errorChan
		}
	}

	if v.TestConfig != nil {
		TestCaseWrapper(v)
	}

	return nil
}

// joinCommands joins the first command with some argument
func joinCommands(cmd, arg string) string {
	cmds := strings.Split(cmd, ",")
	firstCmd := cmds[0] + arg

	if len(cmds) > 1 {
		secondCmd := strings.Join(cmds[1:], ",")
		firstCmd += " " + secondCmd
	}

	return firstCmd
}

// getIPs gets the IPs of the nodes
func getIPs() (ips []string, err error) {
	ips = shared.FetchNodeExternalIP()
	return ips, nil
}

// AddTestCase returns the test case based on the name to be used as customflag.
func AddTestCase(name string) (TestCase, error) {
	if name == "" {
		return func(deployWorkload bool) {}, nil
	}

	testCase := map[string]TestCase{
		"TestDaemonset":                   testcase.TestDaemonset,
		"TestIngress":                     testcase.TestIngress,
		"TestDnsAccess":                   testcase.TestDnsAccess,
		"TestLocalPathProvisionerStorage": testcase.TestLocalPathProvisionerStorage,
		"TestServiceClusterIp":            testcase.TestServiceClusterIp,
		"TestServiceNodePort":             testcase.TestServiceNodePort,
		"TestServiceLoadBalancer":         testcase.TestServiceLoadBalancer,
	}

	if test, ok := testCase[name]; ok {
		return test, nil
	} else {
		return nil, fmt.Errorf("invalid test case name")
	}
}
