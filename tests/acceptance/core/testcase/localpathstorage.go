package testcase

import (
	"fmt"
	"time"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var volumeTest = "volume-test"

func TestLocalPathProvisionerStorage(deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload(
			"create",
			"local-path-provisioner.yaml",
			*util.Arch,
		)
		Expect(err).NotTo(HaveOccurred(),
			"local-path-provisioner manifest not deployed")
	}

	getPodVolumeTestRunning := "kubectl get pods -l app=volume-test" +
		" --field-selector=status.phase=Running --kubeconfig="
	err := assert.ValidateOnHost(
		getPodVolumeTestRunning+util.KubeConfigFile,
		util.Running,
	)
	if err != nil {
		GinkgoT().Errorf("%v", err)
	}

	_, err = util.WriteDataPod(volumeTest)
	if err != nil {
		GinkgoT().Errorf("error writing data to pod: %v", err)
		return
	}

	Eventually(func(g Gomega) {
		fmt.Println("Writing and reading data from pod")
		res, err := util.ReadDataPod(volumeTest)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(res).Should(ContainSubstring("testing local path"))
		g.Expect(err).NotTo(HaveOccurred())
	}, "420s", "2s").Should(Succeed())

	ips := util.FetchNodeExternalIP()
	for _, ip := range ips {
		_, err = util.RestartCluster(ip)
		if err != nil {
			return
		}
	}
	time.Sleep(30 * time.Second)

	_, err = util.ReadDataPod(volumeTest)
	if err != nil {
		return
	}

	err = readDataAfterDeletePod()
	if err != nil {
		return
	}
}

func readDataAfterDeletePod() error {
	deletePod := "kubectl delete pod -l app=volume-test --kubeconfig="

	err := assert.ValidateOnHost(deletePod+util.KubeConfigFile, "deleted")
	if err != nil {
		return err
	}
	time.Sleep(160 * time.Second)

	fmt.Println("Read data from newly create pod")
	_, err = util.ReadDataPod(volumeTest)
	if err != nil {
		return err
	}

	return nil
}
