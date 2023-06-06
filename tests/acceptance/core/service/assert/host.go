package assert

import (
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/tests/acceptance/shared"

	. "github.com/onsi/gomega"
)

// CheckComponentCmdHost runs a command on the host and asserts that the value received contains the specified substring
//
// you can send multiple asserts from a cmd but all of them must be true
//
// need to send sKubeconfigFile
func CheckComponentCmdHost(cmd string, asserts ...string) {
	Eventually(func() error {
		fmt.Println("Executing cmd: ", cmd)
		res, err := shared.RunCommandHost(cmd)
		if err != nil {
			return fmt.Errorf("error on RunCommandHost: %v", err)
		}

		for _, assert := range asserts {
			if !strings.Contains(res, assert) {
				return fmt.Errorf("expected substring %q not found in result %q", assert, res)
			}
			fmt.Println("Result:", res+"\nMatched with assert:", assert)
		}
		return nil
	}, "280s", "5s").Should(Succeed())
}
