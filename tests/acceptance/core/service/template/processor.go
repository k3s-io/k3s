package template

import (
	"fmt"
	"sync"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
)

// processTestCombination run tests using CmdOnNode and CmdOnHost validation and spawn a go routine per ip.
func processTestCombination(resultChan chan error, ips []string, testCombination RunCmd) {
	var wg sync.WaitGroup

	for _, ip := range ips {
		if testCombination.RunOnHost != nil {
			for _, test := range testCombination.RunOnHost {
				wg.Add(1)
				go func(ip string, cmd, expectedValue string) {
					defer wg.Done()
					defer GinkgoRecover()
					processOnHost(resultChan, ip, cmd, expectedValue)
				}(ip, test.Cmd, test.ExpectedValue)
			}
		}

		if testCombination.RunOnNode != nil {
			for _, test := range testCombination.RunOnNode {
				wg.Add(1)
				go func(ip string, cmd, expectedValue string) {
					defer wg.Done()
					defer GinkgoRecover()
					processOnNode(resultChan, ip, cmd, expectedValue)
				}(ip, test.Cmd, test.ExpectedValue)
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
