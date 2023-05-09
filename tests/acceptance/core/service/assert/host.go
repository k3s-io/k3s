package assert

import (
	"fmt"
	"strings"

	util2 "github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/gomega"
)

// CheckComponentCmdHost runs a command on the host and asserts that the value received
// contains the specified substring
// need to send sKubeconfigFile
func CheckComponentCmdHost(cmds []string, asserts ...string) error {
	gomega.Eventually(func() error {
		for _, cmd := range cmds {
			fmt.Printf("Executing cmd: %s\n", cmd)
			res, err := util2.RunCommandHost(cmd)
			if err != nil {
				err = util2.K3sError{
					ErrorSource: cmd,
					Message:     res,
					Err:         err,
				}
				return err
			}
			for _, assert := range asserts {
				fmt.Printf("Checking assert: %s\n", assert)
				if !strings.Contains(res, assert) {
					return fmt.Errorf("expected substring %q not found in result %q", assert, res)
				}
			}
		}
		return nil
	}, "60s", "5s").Should(gomega.Succeed())

	return nil
}
