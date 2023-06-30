package template

import (
	"fmt"
	"strings"
	"sync"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
)

// processTestCombination run tests using CmdOnNode and CmdOnHost validation and spawn a go routine per ip.
func processTestCombination(resultChan chan error, wg *sync.WaitGroup, ips []string, testCombination RunCmd) {
	for _, ip := range ips {
		if testCombination.Run != nil {
			for _, testMap := range testCombination.Run {
				if strings.Contains(testMap.Cmd, "kubectl") {
					wg.Add(1)
					go func(ip string, cmd, expectedValue, expectedValueUpgraded string) {
						defer wg.Done()
						defer GinkgoRecover()
						processOnHost(resultChan, ip, cmd, expectedValue)
					}(ip, testMap.Cmd, testMap.ExpectedValue, testMap.ExpectedValueUpgrade)
				} else {
					wg.Add(1)
					go func(ip string, cmd, expectedValue string) {
						defer wg.Done()
						defer GinkgoRecover()
						processOnNode(resultChan, ip, cmd, expectedValue)
					}(ip, testMap.Cmd, testMap.ExpectedValue)
				}
			}
		}
	}
}

// processOnNode runs the test on the node calling ValidateOnNode
func processOnNode(resultChan chan error, ip, cmd, expectedValue string) {
	if expectedValue == "" {
		err := fmt.Errorf("\nexpected value should be sent")
		fmt.Println("error:", err)
		resultChan <- err
		close(resultChan)
		return
	}

	version := shared.GetK3sVersion()
	fmt.Printf("\nChecking version running on node: %s on ip: %s \n "+
		"Command: %s \nExpected Value: %s\n", version, ip, cmd, expectedValue)

	err := assert.ValidateOnNode(
		ip,
		cmd,
		expectedValue,
	)
	if err != nil {
		fmt.Println("error:", err)
		resultChan <- err
		close(resultChan)
		return
	}
}

// processOnHost runs the test on the host calling ValidateOnHost
func processOnHost(resultChan chan error, ip, cmd, expectedValue string) {
	if expectedValue == "" {
		err := fmt.Errorf("\nexpected value should be sent")
		fmt.Println("error:", err)
		resultChan <- err
		close(resultChan)
		return
	}

	kubeconfigFlag := " --kubeconfig=" + shared.KubeConfigFile
	fullCmd := joinCommands(cmd, kubeconfigFlag)

	version := shared.GetK3sVersion()
	fmt.Printf("\nChecking version running on host: %s on ip: %s \n "+
		"Command: %s \nExpected Value: %s\n", version, ip, fullCmd, expectedValue)

	err := assert.ValidateOnHost(
		fullCmd,
		expectedValue,
	)
	if err != nil {
		fmt.Println("error:", err)
		resultChan <- err
		close(resultChan)
		return
	}
}
