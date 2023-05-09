package template

import (
	"fmt"
	"sync"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	g2 "github.com/onsi/ginkgo/v2"
)

// processTests runs the tests per ips using CmdOnNode and CmdOnHost validation
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
		close(resultChan)
		return
	}

	version := util.GetK3sVersion()
	fmt.Printf("\n Checking version running on node: %s on ip: %s \n "+
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
		close(resultChan)
		return
	}

	kubeconfigFlag := " --kubeconfig=" + util.KubeConfigFile
	cmdResult := joinCommands(cmd, kubeconfigFlag)

	version := util.GetK3sVersion()
	fmt.Printf("\n Checking version running on host: %s on ip: %s \n "+
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
