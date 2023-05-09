package testcase

import (
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// TestPodStatus test the status of the pods in the cluster using 2 custom assert functions
func TestPodStatus(
	g ginkgo.GinkgoTInterface,
	podAssertRestarts assert.PodAssertFunc,
	podAssertReady assert.PodAssertFunc,
	podAssertStatus assert.PodAssertFunc,

) {
	fmt.Printf("\nFetching pod status\n")

	gomega.Eventually(func(g gomega.Gomega) {
		pods, err := util.ParsePods(false)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		for _, pod := range pods {
			if strings.Contains(pod.Name, "helm-install") {
				g.Expect(pod.Status).Should(gomega.Equal("Completed"), pod.Name)
			} else if strings.Contains(pod.Name, "apply") {
				g.Expect(pod.Status).Should(gomega.SatisfyAny(
					gomega.ContainSubstring("Error"),
					gomega.Equal("Completed"),
				), pod.Name)
			} else {
				g.Expect(pod.Status).Should(gomega.Equal("Running"), pod.Name)
				if podAssertRestarts != nil {
					podAssertRestarts(g, pod)
				}
				if podAssertReady != nil {
					podAssertReady(g, pod)
				}
				if podAssertStatus != nil {
					podAssertStatus(g, pod)
				}
			}
		}
	}, "600s", "5s").Should(gomega.Succeed())
}
