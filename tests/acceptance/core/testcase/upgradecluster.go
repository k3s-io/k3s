package testcase

import (
	"fmt"
	"strings"
	"sync"

	util2 "github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestUpgradeCluster(g ginkgo.GinkgoTestingT) {
	ServerIPs := strings.Split(util2.ServerIPs, ",")

	for _, ip := range ServerIPs {
		cmd := "sudo sed -i \"s/|/| INSTALL_K3S_VERSION=" + *util2.UpgradeVersion +
			"/g\" /tmp/master_cmd"
		gomega.Eventually(func(g gomega.Gomega) {
			_, err := util2.RunCmdOnNode(cmd, ip)
			g.Expect(err).NotTo(gomega.HaveOccurred())
		}, "420s", "2s").Should(gomega.Succeed())

		cmd = "sudo chmod u+x /tmp/master_cmd && sudo /tmp/master_cmd"
		gomega.Eventually(func(g gomega.Gomega) {
			_, err := util2.RunCmdOnNode(cmd, ip)
			g.Expect(err).NotTo(gomega.HaveOccurred())
		}, "420s", "2s").Should(gomega.Succeed())
	}

	AgentIPs := strings.Split(util2.AgentIPs, ",")
	for _, ip := range AgentIPs {
		cmd := "sudo sed -i \"s/|/| INSTALL_K3S_VERSION=" + *util2.UpgradeVersion +
			"/g\" /tmp/agent_cmd"
		gomega.Eventually(func(g gomega.Gomega) {
			_, err := util2.RunCmdOnNode(cmd, ip)
			g.Expect(err).NotTo(gomega.HaveOccurred())
		}, "420s", "2s").Should(gomega.Succeed())

		cmd = "sudo chmod u+x /tmp/agent_cmd && sudo /tmp/agent_cmd"
		gomega.Eventually(func(g gomega.Gomega) {
			_, err := util2.RunCmdOnNode(cmd, ip)
			g.Expect(err).NotTo(gomega.HaveOccurred())
		}, "420s", "2s").Should(gomega.Succeed())
	}
}

// TestUpgradeClusterManually upgrades the cluster in "run time".
func TestUpgradeClusterManually(installType string) error {
	serverIPs := strings.Split(util2.ServerIPs, ",")
	agentIPs := strings.Split(util2.AgentIPs, ",")

	err := upgradeServers(installType, serverIPs)
	if err != nil {
		return err
	}

	err = upgradeAgents(installType, agentIPs)
	if err != nil {
		return util2.K3sError{
			ErrorSource: "TestUpgradeClusterManually",
			Message:     "error upgrading cluster in run time",
			Err:         fmt.Errorf("error: %v", err),
		}
	}

	return nil
}

// UpgradeServers upgrades the servers in the cluster.
func upgradeServers(installType string, serverIPs []string) error {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, ip := range serverIPs {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			defer ginkgo.GinkgoRecover()

			cmd := fmt.Sprintf(util2.InstallK3sServer, installType)
			fmt.Printf("\nUpgrading server:  " + cmd)
			if _, err := util2.RunCmdOnNode(cmd, ip); err != nil {
				mu.Lock()
				fmt.Println("Error while upgrading server", err)
				mu.Unlock()
				return
			}

			err := util2.RestartCluster(ip)
			if err != nil {
				mu.Lock()
				fmt.Println("Error while Restarting server", err)
				mu.Unlock()
				return
			}
		}(ip)
	}
	wg.Wait()

	return nil
}

// UpgradeAgents upgrades the agents in the cluster.
func upgradeAgents(installType string, agentIPs []string) error {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, ip := range agentIPs {
		cmd := fmt.Sprintf(util2.InstallK3sAgent, installType)
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			defer ginkgo.GinkgoRecover()

			fmt.Printf("\nUpgrading agent:  " + cmd)
			if _, err := util2.RunCmdOnNode(cmd, ip); err != nil {
				mu.Lock()
				fmt.Println("Error while upgrading agent", err)
				mu.Unlock()
				return
			}

			err := util2.RestartCluster(ip)
			if err != nil {
				mu.Lock()
				fmt.Println("Error while Restarting agent", err)
				mu.Unlock()
				return
			}
		}(ip)
	}
	wg.Wait()

	return nil
}
