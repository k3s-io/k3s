package assert

import (
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/gomega"
)

// CheckComponentCmdHost runs a command on the host and asserts that the value received contains the specified substring
// you can send multiple asserts from a cmd but all of them must be true
// need to send sKubeconfigFile
func CheckComponentCmdHost(cmd string, asserts ...string) error {
	gomega.Eventually(func() error {
		fmt.Printf("Executing cmd: %s\n", cmd)
		res, err := util.RunCommandHost(cmd)
		if err != nil {
			err = util.K3sError{
				ErrorSource: cmd,
				Message:     res,
				Err:         err,
			}
		}

		for _, assert := range asserts {
			fmt.Printf("Checking assert: %s\n", assert)
			if !strings.Contains(res, assert) {
				return fmt.Errorf("expected substring %q not found in result %q", assert, res)
			}
		}

		return nil
	}, "400s", "5s").Should(gomega.Succeed())

	return nil
}
