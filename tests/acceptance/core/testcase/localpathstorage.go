package testcase

import (
	"fmt"
	"time"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/assert"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/customflag"
	"github.com/k3s-io/k3s/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var volumeTest = "volume-test"

func TestLocalPathProvisionerStorage(deployWorkload bool) {
	if deployWorkload {
		_, err := shared.ManageWorkload(
			"create",
			"local-path-provisioner.yaml",
			customflag.ServiceFlag.ClusterConfig.Arch.String(),
		)
		Expect(err).NotTo(HaveOccurred(),
			"local-path-provisioner manifest not deployed")
	}

	getPodVolumeTestRunning := "kubectl get pods -l app=volume-test" +
		" --field-selector=status.phase=Running --kubeconfig=" + shared.KubeConfigFile
	err := assert.ValidateOnHost(
		getPodVolumeTestRunning,
		Running,
	)
	if err != nil {
		GinkgoT().Errorf("%v", err)
	}

	_, err = shared.WriteDataPod(volumeTest)
	if err != nil {
		GinkgoT().Errorf("error writing data to pod: %v", err)
		return
	}

	Eventually(func(g Gomega) {
		fmt.Println("Writing and reading data from pod")
		res, err := shared.ReadDataPod(volumeTest)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(res).Should(ContainSubstring("testing local path"))
		g.Expect(err).NotTo(HaveOccurred())
	}, "420s", "2s").Should(Succeed())

	ips := shared.FetchNodeExternalIP()
	for _, ip := range ips {
		_, err = shared.RestartCluster(ip)
		if err != nil {
			return
		}
	}
	time.Sleep(30 * time.Second)

	_, err = shared.ReadDataPod(volumeTest)
	if err != nil {
		return
	}

	err = readData()
	if err != nil {
		return
	}
}

func readData() error {
	deletePod := "kubectl delete pod -l app=volume-test --kubeconfig="
	err := assert.ValidateOnHost(deletePod+shared.KubeConfigFile, "deleted")
	if err != nil {
		return err
	}
	time.Sleep(160 * time.Second)

	fmt.Println("Read data from newly create pod")
	_, err = shared.ReadDataPod(volumeTest)
	if err != nil {
		return err
	}

	return nil
}
