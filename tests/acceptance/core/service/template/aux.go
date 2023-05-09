package template

import (
	"fmt"
	"strings"
	"sync"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/core/testcase"
	util2 "github.com/k3s-io/k3s/tests/acceptance/shared/util"
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

// postUpgrade checks results after upgrade
func postUpgrade(g g2.GinkgoTInterface, template VersionTestTemplate) error {
	return checkVersion(g, template)
}

// preUpgrade checks results before upgrade
func preUpgrade(g g2.GinkgoTInterface, template VersionTestTemplate) error {
	err := checkVersion(g, template)
	if err != nil {
		return err
	}
	return nil
}

// checkVersion checks the version of RKE2 by calling processTests
func checkVersion(
	g g2.GinkgoTInterface,
	v VersionTestTemplate,
) error {
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

// processTests runs the tests per ips using CmdOnNode and CmdOnHost
func processTests(wg *sync.WaitGroup, resultChan chan error, ips []string, testCombination RunCmd) {
	for _, ip := range ips {
		if testCombination.RunOnHost != nil {
			for _, test := range testCombination.RunOnHost {
				wg.Add(1)
				go func(ip string, cmd, expectedValue string) {
					defer wg.Done()
					defer g2.GinkgoRecover()
					processOnHost(resultChan, ip, cmd, expectedValue)
				}(ip, test.Cmd, test.ExpectedValue)
			}
		}
		if testCombination.RunOnNode != nil {
			for _, test := range testCombination.RunOnNode {
				wg.Add(1)
				go func(ip string, cmd, expectedValue string) {
					defer wg.Done()
					defer g2.GinkgoRecover()
					processOnNode(resultChan, ip, cmd, expectedValue)
				}(ip, test.Cmd, test.ExpectedValue)
			}
		}
	}
}

// processOnNode runs the test on the node calling ValidateOnNode
func processOnNode(resultChan chan error, ip, cmd, expectedValue string) {
	if expectedValue == "" {
		err := fmt.Errorf("expected value should be sent")
		fmt.Println("Error:", err)
		resultChan <- err
		return
	}

	version := util2.GetK3sVersion()
	fmt.Printf("\n Checking version: %s on ip: %s \n "+
		"Command: %s \n Expected Value: %s", version, ip, cmd, expectedValue)

	err := assert.ValidateOnNode(
		ip,
		cmd,
		expectedValue,
	)
	if err != nil {
		fmt.Println("Error:", err)
		resultChan <- err
	}
}

// processOnHost runs the test on the host calling ValidateOnHost
func processOnHost(resultChan chan error, ip, cmd, expectedValue string) {
	if expectedValue == "" {
		err := fmt.Errorf("expected value should be sent")
		fmt.Println("Error:", err)
		resultChan <- err
		return
	}

	kubeconfigFlag := " --kubeconfig=" + util2.KubeConfigFile
	cmdResult := joinCommands(cmd, kubeconfigFlag)

	version := util2.GetK3sVersion()
	fmt.Printf("\n Checking version: %s on ip: %s \n "+
		"Command: %s \n Expected Value: %s", version, ip, cmdResult, expectedValue)

	err := assert.ValidateOnHost(
		cmdResult,
		expectedValue,
	)
	if err != nil {
		fmt.Println("Error:", err)
		resultChan <- err
	}
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
	ips = util2.FetchNodeExternalIP()
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
