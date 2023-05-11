package testcase

import (
	"fmt"
	"time"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestLocalPathProvisionerStorage(deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload(
			"create",
			"local-path-provisioner.yaml",
			*util.Arch,
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"local-path-provisioner manifest not deployed")
	}

	err := assert.ValidateOnHost(
		util.GetPodVolumeTestRunning+util.KubeConfigFile,
		util.RunningAssert,
	)
	if err != nil {
		GinkgoT().Errorf("Error: %v", err)
	}

	_, err = util.WriteDataPod(util.VolumeTest)
	if err != nil {
		GinkgoT().Errorf("error writing data to pod: %v", err)
		return
	}

	gomega.Eventually(func(g gomega.Gomega) {
		fmt.Println("Writing and reading data from pod")
		res, err := util.ReadDataPod(util.VolumeTest)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(res).Should(gomega.ContainSubstring(util.TestingLocalPath))
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}, "420s", "2s").Should(gomega.Succeed())

	ips := util.FetchNodeExternalIP()
	for _, ip := range ips {
		_, err = util.RestartCluster(ip)
		if err != nil {
			return
		}
	}
	time.Sleep(30 * time.Second)

	_, err = util.ReadDataPod(util.VolumeTest)
	if err != nil {
		return
	}

	err = readDataAfterDeletePod()
	if err != nil {
		return
	}
}

func readDataAfterDeletePod() error {
	err := assert.ValidateOnHost(util.DeletePod+util.KubeConfigFile, "deleted")
	if err != nil {
		return err
	}
	time.Sleep(160 * time.Second)

	fmt.Println("Read data from newly create pod")
	_, err = util.ReadDataPod(util.VolumeTest)
	if err != nil {
		return err
	}

	return nil
}
