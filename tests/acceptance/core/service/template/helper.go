package template

import (
	"fmt"
	"strings"
	"sync"

	"github.com/k3s-io/k3s/tests/acceptance/core/testcase"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	g2 "github.com/onsi/ginkgo/v2"
)

// upgradeVersion upgrades the version of RKE2 and updates the expected values
func upgradeVersion(g g2.GinkgoTInterface, template VersionTestTemplate, version string) error {
	err := testcase.TestUpgradeClusterManually(version)
	if err != nil {
		return err
	}

	for i := range template.TestCombination.RunOnNode {
		template.TestCombination.RunOnNode[i].ExpectedValue =
			template.TestCombination.RunOnNode[i].ExpectedValueUpgrade
	}

	for i := range template.TestCombination.RunOnHost {
		template.TestCombination.RunOnHost[i].ExpectedValue =
			template.TestCombination.RunOnHost[i].ExpectedValueUpgrade
	}

	return nil
}

// checkVersion checks the version of k3s and processes the tests
func checkVersion(g g2.GinkgoTInterface, v VersionTestTemplate) error {
	ips, err := getIPs()
	if err != nil {
		g2.Fail(fmt.Sprintf("Failed to get IPs: %s", err))
	}

	var wg sync.WaitGroup
	errorChanList := make(
		chan error,
		len(ips)*(len(v.TestCombination.RunOnHost)+len(v.TestCombination.RunOnNode)),
	)

	processTests(&wg, errorChanList, ips, *v.TestCombination)

	wg.Wait()
	close(errorChanList)

	for errorChan := range errorChanList {
		if errorChan != nil {
			return errorChan
		}
	}

	return nil
}

// joinCommands joins split commands by comma and then joins the first command with the flag
func joinCommands(cmd, Flag string) string {
	cmds := strings.Split(cmd, ",")
	firstCmd := cmds[0] + Flag

	if len(cmds) > 1 {
		secondCmd := strings.Join(cmds[1:], ",")
		firstCmd += " " + secondCmd
	}

	return firstCmd
}

// getIPs gets the IPs of the nodes
func getIPs() (ips []string, err error) {
	ips = util.FetchNodeExternalIP()
	return ips, nil
}

// GetTestCase returns the test case based on the name to be used as flag.
func GetTestCase(name string) (TestCase, error) {
	if name == "" {
		return func(g g2.GinkgoTestingT, deployWorkload bool) {}, nil
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
	}

	return nil, fmt.Errorf("invalid test case name")
}
