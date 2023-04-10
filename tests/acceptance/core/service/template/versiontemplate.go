package template

import (
	"fmt"

	g2 "github.com/onsi/ginkgo/v2"
)

func VersionTemplate(g g2.GinkgoTInterface, test VersionTestTemplate) {
	err := preUpgrade(g, test)
	if err != nil {
		g2.Fail(err.Error())
		return
	}

	for _, version := range test.InstallUpgrade {
		if g.Failed() {
			fmt.Println("CheckVersion failed, not proceeding to upgrade")
			return
		}

		err = upgradeVersion(g, test, version)
		if err != nil {
			g2.Fail(fmt.Sprintf("Error upgrading: %v\n", err))
			return
		}

		err = postUpgrade(g, test)
		if err != nil {
			g2.Fail(err.Error())
			return
		}

		if test.TestConfig != nil {
			TestCaseWrapper(g, test)
		}
	}
}
