//go:build versionbump

package versionbump

import (
	"fmt"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/customflag"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/template"
	"github.com/k3s-io/k3s/tests/acceptance/core/testcase"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("VersionTemplate Upgrade:", func() {

	It("Start Up with no issues", func() {
		testcase.TestBuildCluster(GinkgoT())
	})

	It("Checks Node Status", func() {
		testcase.TestNodeStatus(
			assert.NodeAssertReadyStatus(),
			nil)
	})

	It("Checks Pod Status", func() {
		testcase.TestPodStatus(
			assert.PodAssertRestart(),
			assert.PodAssertReady(),
			assert.PodAssertStatus())
	})

	It("Verifies bump version", func() {
		template.VersionTemplate(template.VersionTestTemplate{
			Description: template.TestMapFlag.Description,
			TestCombination: &template.RunCmd{
				RunOnNode: []template.TestMap{
					{
						Cmd:                  template.TestMapFlag.CmdNode,
						ExpectedValue:        template.TestMapFlag.ExpectedValueNode,
						ExpectedValueUpgrade: template.TestMapFlag.ExpectedValueUpgradedNode,
					},
				},
				RunOnHost: []template.TestMap{
					{
						Cmd:                  template.TestMapFlag.CmdHost,
						ExpectedValue:        template.TestMapFlag.ExpectedValueHost,
						ExpectedValueUpgrade: template.TestMapFlag.ExpectedValueUpgradedHost,
					},
				},
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
