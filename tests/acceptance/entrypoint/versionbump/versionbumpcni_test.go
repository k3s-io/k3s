//go:build cniplugin

package versionbump

import (
	"fmt"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/customflag"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/template"
	"github.com/k3s-io/k3s/tests/acceptance/core/testcase"
	"github.com/k3s-io/k3s/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("VersionTemplate Upgrade:", func() {
	It("Start Up with no issues", func() {
		testcase.TestBuildCluster(GinkgoT())
	})

	It("Validate Node", func() {
		testcase.TestNodeStatus(
			assert.NodeAssertReadyStatus(),
			nil)
	})

	It("Validate Pod", func() {
		testcase.TestPodStatus(
			assert.PodAssertRestart(),
			assert.PodAssertReady(),
			assert.PodAssertStatus())
	})

	It("Create bandwidth test pod", func() {
		_, err := shared.ManageWorkload(
			"create",
			"bandwidth-annotations.yaml",
			customflag.ServiceFlag.ClusterConfig.Arch.String(),
		)
		if err != nil {
			fmt.Println("Error creating workload")
		}
	})

	It("Verifies bump version", func() {
		template.VersionTemplate(template.VersionTestTemplate{
			Description: "CNI Plugin Version Bump",
			TestCombination: &template.RunCmd{
				Run: []template.TestMap{
					{
						Cmd:                  "/var/lib/rancher/k3s/data/current/bin/cni",
						ExpectedValue:        template.TestMapFlag.ExpectedValue,
						ExpectedValueUpgrade: template.TestMapFlag.ExpectedValueUpgrade,
					},
				},
				// Run: []template.TestMap{
				// 	{
				// 		Cmd: "kubectl get pod test-pod -o yaml --kubeconfig=" + "," +
				// 			" | grep -A2 annotations ",
				// 		ExpectedValue:        template.TestMapFlag.ExpectedValueHost,
				// 		ExpectedValueUpgrade: template.TestMapFlag.ExpectedValueUpgradedHost,
				// 	},
				// },
			},
			InstallUpgrade: customflag.ServiceFlag.InstallUpgrade,
			TestConfig: &template.TestConfig{
				TestFunc:       template.TestCase(customflag.ServiceFlag.TestCase.TestFunc),
				DeployWorkload: customflag.ServiceFlag.TestCase.DeployWorkload,
			},
		})
	})
})

var _ = AfterEach(func() {
	if CurrentSpecReport().Failed() {
		fmt.Printf("\nFAILED! %s\n", CurrentSpecReport().FullText())
	} else {
		fmt.Printf("\nPASSED! %s\n", CurrentSpecReport().FullText())
	}
})
