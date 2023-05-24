package assert

import (
	"fmt"
	"strings"
	"time"

	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
)

// validate runs a command on host or node and asserts that the value received against his respective command
//
// the first caller - process tests will spawn a go routine per ip the cluster
//
// need to send KubeconfigFile
func validate(exec func(string) (string, error), args ...string) error {
	if len(args) < 2 || len(args)%2 != 0 {
		return fmt.Errorf("must receive an even number of arguments as cmd/assert pairs")
	}

	errorsChan := make(chan error, len(args)/2)
	timeout := time.After(220 * time.Second)
	ticker := time.NewTicker(3 * time.Second)

	for i := 0; i < len(args); i++ {
		cmd := args[i]
		if i+1 < len(args) {
			assert := args[i+1]
			i++

			for {
				select {
				case <-timeout:
					timeoutErr := fmt.Errorf("timeout reached for command:\n%s\n "+
						"Trying to assert with:\n %s",
						cmd, assert)
					errorsChan <- timeoutErr
					close(errorsChan)
					return timeoutErr

				case <-ticker.C:
					res, err := exec(cmd)
					if err != nil {
						errorsChan <- fmt.Errorf("from runCmd\n %s\n %s", res, err)
						close(errorsChan)
						return err
					}

					fmt.Printf("\nCMD: %s\nRESULT: %s\nAssertion: %s\n", cmd, res, assert)
					if strings.Contains(res, assert) {
						fmt.Printf("Matched with: \n%s\n", res)
						errorsChan <- nil
						return nil
					}
				}
			}
		}
	}
	close(errorsChan)

	return nil
}

// ValidateOnHost runs an exec function on RunCommandHost and asserts that the value received
func ValidateOnHost(args ...string) error {
	exec := func(cmd string) (string, error) {
		return util.RunCommandHost(cmd)
	}
	return validate(exec, args...)
}

// ValidateOnNode runs an exec function on RunCommandOnNode and asserts that the value received
func ValidateOnNode(ip string, args ...string) error {
	exec := func(cmd string) (string, error) {
		return util.RunCmdOnNode(cmd, ip)
	}
	return validate(exec, args...)
}
