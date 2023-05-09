package assert

import (
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/gomega"
)

// PodAssertFunc is a function type used to create pod assertions
type PodAssertFunc func(g gomega.Gomega, pod util.Pod)

// PodAssertRestarts custom assertion func that asserts that pods are not restarting with no reason
// controller, scheduler, helm-install pods can be restarted occasionally when cluster started if only once
func PodAssertRestarts() PodAssertFunc {
	return func(g gomega.Gomega, pod util.Pod) {
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
	return func(g gomega.Gomega, pod util.Pod) {
		if strings.Contains(pod.Name, "helm-install") {
			g.Expect(pod.Status).Should(gomega.Equal(util.CompletedAssert), pod.Name)
		} else if strings.Contains(pod.Name, "apply") &&
			strings.Contains(pod.NameSpace, "system-upgrade") {
			g.Expect(pod.Status).Should(gomega.SatisfyAny(
				gomega.ContainSubstring("Error"),
				gomega.Equal(util.CompletedAssert),
			), pod.Name)
		} else {
			g.Expect(pod.Status).Should(gomega.Equal(util.RunningAssert), pod.Name)
		}
	}
}

// PodAssertReady custom assertion func that asserts that the pod is
// with correct numbers of ready containers
func PodAssertReady() PodAssertFunc {
	return func(g gomega.Gomega, pod util.Pod) {
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

// CheckPodStatusRunning asserts that the pod is running with the specified label = app name.
// don't need to send sKubeconfigFile
func CheckPodStatusRunning(name, assert string) error {
	cmd := "kubectl get pods -l k8s-app=" + name +
		" --field-selector=status.phase=Running --kubeconfig=" + util.KubeConfigFile
	gomega.Eventually(func(g gomega.Gomega) {
		res, err := util.RunCommandHost(cmd)
		if err != nil {
			err = util.K3sError{
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
