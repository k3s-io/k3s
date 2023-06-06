package template

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
)

func VersionTemplate(test VersionTestTemplate) {
	err := checkVersion(test)
	if err != nil {
		GinkgoT().Errorf(err.Error())
		return
	}

	for _, version := range test.InstallUpgrade {
		if GinkgoT().Failed() {
			fmt.Println("checkVersion failed, not proceeding to upgrade")
			return
		}

		upgErr := upgradeVersion(test, version)
		if upgErr != nil {
			GinkgoT().Errorf("error upgrading: %v\n", err)
			return
		}

		err = checkVersion(test)
		if err != nil {
			GinkgoT().Errorf(err.Error())
			return
		}

		if test.TestConfig != nil {
			TestCaseWrapper(test)
		}
	}
}
