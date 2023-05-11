package template

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
)

func VersionTemplate(g GinkgoTInterface, test VersionTestTemplate) {
	err := checkVersion(g, test)
	if err != nil {
		GinkgoT().Fatalf(err.Error())
		return
	}

	for _, version := range test.InstallUpgrade {
		if g.Failed() {
			fmt.Println("CheckVersion failed, not proceeding to upgrade")
			return
		}

		err = upgradeVersion(test, version)
		if err != nil {
			GinkgoT().Fatalf("Error upgrading: %v\n", err)
			return
		}

		err = checkVersion(g, test)
		if err != nil {
			GinkgoT().Fatalf(err.Error())
			return
		}

		if test.TestConfig != nil {
			TestCaseWrapper(test)
		}
	}
}
