//go:build cniplugin

package versionbump

import (
	"fmt"

	"github.com/k3s-io/k3s/tests/acceptance/core/service"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/customflag"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/template"
	"github.com/k3s-io/k3s/tests/acceptance/core/testcase"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("VersionTemplate Upgrade:", func() {
	It("Start Up with no issues", func() {
		testcase.TestBuildCluster(GinkgoT(), false)
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

	It("Create bandwidth test pod", func() {
		_, err := util.ManageWorkload(
			"create",
			"bandwidth-annotations.yaml",
			*util.Arch,
		)
		if err != nil {
			fmt.Println("Error creating workload")
		}
	})

	It("Verifies bump version", func() {
		template.VersionTemplate(GinkgoT(), template.VersionTestTemplate{
			Description: "CNI Plugin Version Bump",
			TestCombination: &template.RunCmd{
				RunOnNode: []template.TestMap{
					{
						Cmd:                  util.CNIbin,
						ExpectedValue:        service.ExpectedValueNode,
						ExpectedValueUpgrade: service.ExpectedValueUpgradedNode,
					},
					{
						Cmd:                  util.FlannelBinVersion,
						ExpectedValue:        service.ExpectedValueNode,
						ExpectedValueUpgrade: service.ExpectedValueUpgradedHost,
					},
				},
				RunOnHost: []template.TestMap{
					{
						Cmd:                  util.GetPodTestWithAnnotations + "," + util.GrepAnnotations,
						ExpectedValue:        service.ExpectedValueHost,
						ExpectedValueUpgrade: service.ExpectedValueUpgradedHost,
					},
				},
			},
			InstallUpgrade: customflag.InstallUpgradeFlag,
			TestConfig:     nil,
		})
	})
})

var _ = BeforeEach(func() {
	if *util.Destroy {
		Skip("Cluster is being Deleted")
	}
})

var _ = AfterEach(func() {
	if CurrentSpecReport().Failed() {
		fmt.Printf("\nFAILED! %s\n", CurrentSpecReport().FullText())
	} else {
		fmt.Printf("\nPASSED! %s\n", CurrentSpecReport().FullText())
	}
})
