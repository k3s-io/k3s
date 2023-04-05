package rotatecertificate

import "github.com/k3s-io/k3s/tests/e2e"

// rotateCertificate rotate the Certificate on each node given
func rotateCertificate(nodeNames []string) error {
	for _, nodeName := range nodeNames {
		cmd := "sudo k3s --debug certificate rotate"
		if _, err := e2e.RunCmdOnNode(cmd, nodeName); err != nil {
			return err
		}
	}
	return nil
}
