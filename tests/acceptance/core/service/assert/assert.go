package assert

import (
	"fmt"
	"strings"
	"sync"
	"time"

	util2 "github.com/k3s-io/k3s/tests/acceptance/shared/util"
	g2 "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

type CommandAssert struct {
	Command string
	Assert  string
}

type PodAssertFunc func(g gomega.Gomega, pod util2.Pod)
type NodeAssertFunc func(g gomega.Gomega, node util2.Node)

// PodAssertRestarts custom assertion func that asserts that pods are not restarting with no reason
// controller, scheduler, helm-install pods can be restarted occasionally when cluster started if only once
func PodAssertRestarts() PodAssertFunc {
	return func(g gomega.Gomega, pod util2.Pod) {
		if strings.Contains(pod.NameSpace, "kube-system") &&
			strings.Contains(pod.Name, "controller") &&
			strings.Contains(pod.Name, "scheduler") {
			g.Expect(pod.Restarts).Should(gomega.SatisfyAny(gomega.Equal("0"),
				gomega.Equal("1")),
				"could be restarted occasionally when cluster started", pod.Name)
		}
	}
}

// PodAssertStatus custom assertion that asserts that pods status is completed or in some cases
// apply pods can have an error status
func PodAssertStatus() PodAssertFunc {
	return func(g gomega.Gomega, pod util2.Pod) {
		if strings.Contains(pod.Name, "helm-install") {
			g.Expect(pod.Status).Should(gomega.Equal(util2.CompletedAssert), pod.Name)
		} else if strings.Contains(pod.Name, "apply") &&
			strings.Contains(pod.NameSpace, "system-upgrade") {
			g.Expect(pod.Status).Should(gomega.SatisfyAny(
				gomega.ContainSubstring("Error"),
				gomega.Equal(util2.CompletedAssert),
			), pod.Name)
		} else {
			g.Expect(pod.Status).Should(gomega.Equal(util2.RunningAssert), pod.Name)
		}
	}
}

// PodAssertReady custom assertion func that asserts that the pod is
// with correct numbers of ready containers
func PodAssertReady() PodAssertFunc {
	return func(g gomega.Gomega, pod util2.Pod) {
		g.ExpectWithOffset(1, pod.Ready).To(checkReadyFields(),
			"should have equal values in n/n format")
	}
}

// checkReadyFields is a custom matcher that checks
// if the input string is in N/N format and the same qty
func checkReadyFields() gomega.OmegaMatcher {
	return gomega.WithTransform(func(s string) (bool, error) {
		var a, b int
		n, err := fmt.Sscanf(s, "%d/%d", &a, &b)
		if err != nil || n != 2 {
			return false, fmt.Errorf("failed to parse format: %v", err)
		}
		return a == b, nil
	}, gomega.BeTrue())
}

// NodeAssertVersionUpgraded  custom assertion func that asserts that node
// is upgraded to the specified version
func NodeAssertVersionUpgraded() NodeAssertFunc {
	return func(g gomega.Gomega, node util2.Node) {
		g.Expect(node.Version).Should(gomega.Equal(*util2.UpgradeVersion),
			"Nodes should all be upgraded to the specified version", node.Name)
	}
}

// NodeAssertReadyStatus custom assertion func that asserts that node is Ready
func NodeAssertReadyStatus() NodeAssertFunc {
	return func(g gomega.Gomega, node util2.Node) {
		g.Expect(node.Status).Should(gomega.Equal("Ready"),
			"Nodes should all be in Ready state")
	}
}

// NodeAssertCount custom assertion func that asserts that node count is as expected
func NodeAssertCount() NodeAssertFunc {
	return func(g gomega.Gomega, node util2.Node) {
		expectedNodeCount := util2.NumServers + util2.NumAgents
		nodes, err := util2.ParseNodes(false)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(len(nodes)).To(gomega.Equal(expectedNodeCount),
			"Number of nodes should match the spec")
	}
}

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

// CheckComponentCmdNode runs a command on a node and asserts that the value received
// contains the specified substring
func CheckComponentCmdNode(cmd string, ip string, assert string) {
	gomega.Eventually(func(g gomega.Gomega) {
		res, err := util2.RunCmdOnNode(cmd, ip)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(res).Should(gomega.ContainSubstring(assert))
	}, "420s", "5s").Should(gomega.Succeed())
}

// CheckPodStatusRunning asserts that the pod is running with the specified label = app name.
// don't need to send sKubeconfigFile
func CheckPodStatusRunning(name, assert string) error {
	cmd := "kubectl get pods -l k8s-app=" + name +
		" --field-selector=status.phase=Running --kubeconfig=" + util2.KubeConfigFile
	gomega.Eventually(func(g gomega.Gomega) {
		res, err := util2.RunCommandHost(cmd)
		if err != nil {
			err = util2.K3sError{
				ErrorSource: cmd,
				Message:     res,
				Err:         err,
			}
			return
		}
		g.Expect(res).Should(gomega.ContainSubstring(assert))
	}, "420s", "5s").Should(gomega.Succeed())

	return nil
}

// validate runs a command on a node and asserts that the value received
// need to sent sKubeconfigFile
func validate(exec func(string) (string, error), args ...string) error {
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

				timeout := time.After(420 * time.Second)
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
		return util2.RunCommandHost(cmd)
	}
	return validate(exec, args...)
}

// ValidateOnNode runs an exec function on RunCommandOnNode and asserts that the value received
// calling RunCommandOnNode
func ValidateOnNode(ip string, args ...string) error {
	exec := func(cmd string) (string, error) {
		return util2.RunCmdOnNode(cmd, ip)
	}
	return validate(exec, args...)
}
