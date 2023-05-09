package assert

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	g2 "github.com/onsi/ginkgo/v2"
)

// Validate runs a command on host or node and asserts that the value received against his respective command
// when called by node function it will spawn go routines for each ip in the cluster
// need to sent KubeconfigFile
func Validate(exec func(string) (string, error), args ...string) error {
	if len(args) < 2 || len(args)%2 != 0 {
		return fmt.Errorf("must receive an even number of arguments as cmd/assert pairs")
	}

	var wg sync.WaitGroup
	errorsChan := make(chan error, len(args)/2)

	for i := 0; i < len(args); i++ {
		cmd := args[i]
		if i+1 < len(args) {
			assert := args[i+1]
			i++

			wg.Add(1)
			go func(cmd, assert string) {
				defer wg.Done()
				defer g2.GinkgoRecover()

				timeout := time.After(620 * time.Second)
				ticker := time.NewTicker(3 * time.Second)

				for {
					select {
					case <-timeout:
						errorTimeout := fmt.Errorf("timeout reached for command: %s", cmd)
						errorsChan <- errorTimeout
						fmt.Println("timeout reached for command: \n Trying to assert with:", cmd, assert)
						close(errorsChan)
						return
					case <-ticker.C:
						res, err := exec(cmd)
						if err != nil {
							fmt.Println("error from RunCmd:\n", res, "\n", err)
							close(errorsChan)
							return
						}
						fmt.Printf("\nCMD: %s\nRESULT: %s\nAssertion: %s\n", cmd, res, assert)
						if strings.Contains(res, assert) {
							fmt.Printf("Matched with: \n%s\n", res)
							errorsChan <- nil
							return
						}
					}
				}
			}(cmd, assert)
		}
	}
	wg.Wait()
	close(errorsChan)

	return nil
}

// ValidateOnHost runs an exec function on RunCommandHost and asserts that the value received
// calling RunCommandHost
func ValidateOnHost(args ...string) error {
	exec := func(cmd string) (string, error) {
		return util.RunCommandHost(cmd)
	}
	return Validate(exec, args...)
}

// ValidateOnNode runs an exec function on RunCommandOnNode and asserts that the value received
// calling RunCommandOnNode
func ValidateOnNode(ip string, args ...string) error {
	exec := func(cmd string) (string, error) {
		return util.RunCmdOnNode(cmd, ip)
	}
	return Validate(exec, args...)
}
