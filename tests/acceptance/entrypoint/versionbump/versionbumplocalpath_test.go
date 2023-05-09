//go:build localpath

package versionbump

import (
	"fmt"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
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
		testcase.TestNodeStatus(GinkgoT(),
			assert.NodeAssertReadyStatus(),
			nil)
	})

	It("Checks Pod Status", func() {
		testcase.TestPodStatus(GinkgoT(),
			assert.PodAssertRestarts(),
			assert.PodAssertReady(),
			assert.PodAssertStatus())
	})

	It("Verifies bump local path storage version", func() {
		template.VersionTemplate(GinkgoT(), template.VersionTestTemplate{
			Description: util.Description,
			TestCombination: &template.RunCmd{
				RunOnNode: []template.TestMap{
					{
						Cmd:                  util.K3sVersion,
						ExpectedValue:        util.ExpectedValueNode,
						ExpectedValueUpgrade: util.ExpectedValueUpgradedNode,
					},
				},
				RunOnHost: []template.TestMap{
					{
						Cmd:                  util.GetImageLocalPath + "," + util.GrepImage,
						ExpectedValue:        util.ExpectedValueHost,
						ExpectedValueUpgrade: util.ExpectedValueUpgradedHost,
					},
				},
			},
			InstallUpgrade: util.InstallUpgradeFlag,
			TestConfig: &template.TestConfig{
				TestFunc:       testcase.TestLocalPathProvisionerStorage,
				DeployWorkload: true,
			},
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
