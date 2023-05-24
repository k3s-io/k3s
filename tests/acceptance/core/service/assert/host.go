package assert

import (
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/gomega"
)

// CheckComponentCmdHost runs a command on the host and asserts that the value received contains the specified substring
//
// you can send multiple asserts from a cmd but all of them must be true
//
// need to send sKubeconfigFile
func CheckComponentCmdHost(cmd string, asserts ...string) {
	gomega.Eventually(func() error {
		fmt.Printf("\nExecuting cmd: %s\n", cmd)
		res, err := util.RunCommandHost(cmd)
		if err != nil {
			return err
		}

		for _, assert := range asserts {
			if !strings.Contains(res, assert) {
				return fmt.Errorf("expected substring %q not found in result %q", assert, res)
			}
			fmt.Printf("Matches with assert: %s \n", assert)
		}
		return nil
	}, "180s", "5s").Should(gomega.Succeed())
}
